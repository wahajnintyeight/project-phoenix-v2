#!/usr/bin/env python3
"""Re-embed a Qdrant collection into a new collection with a different dimension.

The script preserves point IDs and payloads. It deliberately regenerates vectors
from payload text; copying 384-dimensional source vectors into a 1536-dimensional
collection is invalid and would not improve semantic search.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
import time
from pathlib import Path
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.parse import quote, urlencode, urlparse, urlunparse
from urllib.request import Request, urlopen


DEFAULT_SOURCE = "activity_tracker_memories"
DEFAULT_TARGET = "activity_tracker_memories_v2"
DEFAULT_VECTOR_NAME = "activity_tracker_memories_v2"
DEFAULT_DIMENSION = 1536
TEXT_FIELDS = (
    "summary",
    "content",
    "text",
    "activity",
    "application",
    "category",
    "user_state",
    "observations",
    "description",
)


def load_dotenv(path: Path = Path(".env")) -> None:
    """Load simple KEY=VALUE lines without overriding the process environment."""
    if not path.is_file():
        return
    for raw_line in path.read_text(encoding="utf-8").splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        key = key.strip()
        value = value.strip()
        if value[:1] in ('"', "'") and value[-1:] == value[:1]:
            value = value[1:-1]
        if key and key not in os.environ:
            os.environ[key] = value


def http_json(
    url: str,
    *,
    method: str = "GET",
    body: dict[str, Any] | None = None,
    headers: dict[str, str] | None = None,
    timeout: int = 90,
    retry_http: bool = True,
) -> dict[str, Any]:
    data = None if body is None else json.dumps(body).encode("utf-8")
    request_headers = {"Accept": "application/json"}
    if data is not None:
        request_headers["Content-Type"] = "application/json"
    if headers:
        request_headers.update(headers)
    request = Request(url, data=data, headers=request_headers, method=method)

    for attempt in range(8):
        try:
            with urlopen(request, timeout=timeout) as response:
                raw = response.read()
            return json.loads(raw) if raw else {}
        except HTTPError as exc:
            detail = exc.read().decode("utf-8", errors="replace")
            if retry_http and exc.code in (429, 500, 502, 503, 504) and attempt < 7:
                retry_after = exc.headers.get("Retry-After")
                if retry_after:
                    delay = float(retry_after)
                else:
                    retry_hint = re.search(r'"retryDelay"\s*:\s*"([0-9.]+)s"', detail)
                    if not retry_hint:
                        retry_hint = re.search(r"retry in ([0-9.]+)s", detail, re.IGNORECASE)
                    delay = float(retry_hint.group(1)) if retry_hint else min(2**attempt, 30)
                delay = min(max(delay, 1), 300)
                print(f"HTTP {exc.code}; retrying in {delay:g}s", file=sys.stderr)
                time.sleep(delay)
                continue
            raise APIError(exc.code, detail) from None
        except (URLError, TimeoutError) as exc:
            if attempt == 5:
                raise RuntimeError(f"Request to {urlparse(url).netloc} failed: {exc}") from exc
            delay = min(2**attempt, 30)
            print(f"Network error; retrying in {delay}s", file=sys.stderr)
            time.sleep(delay)
    raise RuntimeError("Request retries exhausted")


class APIError(RuntimeError):
    def __init__(self, status: int, detail: str):
        self.status = status
        self.detail = detail
        super().__init__(f"HTTP {status}: {detail}")


def load_google_keys() -> list[str]:
    helper = Path(__file__).resolve().parent / "migration_google_keys"
    result = subprocess.run(
        ["go", "run", "-mod=mod", str(helper)], capture_output=True, text=True, timeout=180, check=False
    )
    if result.returncode:
        raise RuntimeError(f"Unable to load Gemini keys from MongoDB: {result.stderr.strip()}")
    keys = json.loads(result.stdout)
    if not keys:
        raise RuntimeError("MongoDB has no Google keys with status Valid")
    return keys


class EmbeddingClient:
    def __init__(self, provider: str, dimension: int):
        self.provider = provider
        self.dimension = dimension
        self.model = os.getenv("EMBEDDING_MODEL", "").strip()
        self.api_key = ""
        self.google_keys: list[str] = []
        self.google_index = 0
        self.google_cooldowns: dict[int, float] = {}
        self.google_disabled: set[int] = set()

        if provider == "google":
            self.google_keys = load_google_keys()
            self.api_key = self.google_keys[0]
            self.model = self.model or "gemini-embedding-2"
            print(f"Loaded {len(self.google_keys)} valid Google keys from MongoDB.", flush=True)
        elif provider == "openai":
            self.api_key = os.getenv("OPENAI_API_KEY", "").strip()
            self.model = self.model or "text-embedding-3-small"
        elif provider == "cohere":
            self.api_key = os.getenv("COHERE_API_KEY", "").strip()
            self.model = self.model or "embed-v5.0-pro"

        if not self.api_key:
            key_name = {"google": "GOOGLE_API_KEY or GEMINI_API_KEY", "openai": "OPENAI_API_KEY", "cohere": "COHERE_API_KEY"}[provider]
            raise RuntimeError(f"{key_name} is required for the {provider} embedding API")

    def embed(self, texts: list[str]) -> list[list[float]]:
        if self.provider == "google":
            requests = [
                {
                    "model": f"models/{self.model}",
                    "content": {"parts": [{"text": text}]},
                    "outputDimensionality": self.dimension,
                    "taskType": "RETRIEVAL_DOCUMENT",
                }
                for text in texts
            ]
            while True:
                now = time.monotonic()
                available = [i for i in range(len(self.google_keys)) if i not in self.google_disabled and self.google_cooldowns.get(i, 0) <= now]
                if not available:
                    active = [i for i in range(len(self.google_keys)) if i not in self.google_disabled]
                    if not active:
                        raise RuntimeError("All MongoDB Google keys were rejected by the embedding API")
                    wait_for = max(1, min(self.google_cooldowns[i] for i in active) - now)
                    print(f"All Google keys are cooling down; waiting {wait_for:.0f}s.", flush=True)
                    time.sleep(wait_for)
                    continue
                candidates = [i for i in available if i >= self.google_index]
                index = candidates[0] if candidates else available[0]
                self.google_index = (index + 1) % len(self.google_keys)
                try:
                    response = http_json(
                        f"https://generativelanguage.googleapis.com/v1beta/models/{quote(self.model)}:batchEmbedContents",
                        method="POST",
                        headers={"x-goog-api-key": self.google_keys[index]},
                        body={"requests": requests},
                        retry_http=False,
                    )
                    break
                except APIError as exc:
                    if exc.status == 429:
                        hint = re.search(r'"retryDelay"\s*:\s*"([0-9.]+)s"', exc.detail)
                        delay = max(60.0, float(hint.group(1)) + 1) if hint else 60.0
                        self.google_cooldowns[index] = time.monotonic() + min(delay, 3600)
                        print(f"Google key {index + 1}/{len(self.google_keys)} rate-limited; trying another available key.", flush=True)
                    elif exc.status in (401, 403) or (exc.status == 400 and "API_KEY_INVALID" in exc.detail):
                        self.google_disabled.add(index)
                        print(f"Google key {index + 1}/{len(self.google_keys)} rejected; trying another key.", flush=True)
                    elif exc.status in (500, 502, 503, 504):
                        self.google_cooldowns[index] = time.monotonic() + 30
                        print(f"Google returned HTTP {exc.status}; trying another available key.", flush=True)
                    else:
                        raise
            vectors = [item["values"] for item in response.get("embeddings", [])]
        elif self.provider == "openai":
            response = http_json(
                "https://api.openai.com/v1/embeddings",
                method="POST",
                headers={"Authorization": f"Bearer {self.api_key}"},
                body={"model": self.model, "input": texts, "dimensions": self.dimension, "encoding_format": "float"},
            )
            data = sorted(response.get("data", []), key=lambda item: item["index"])
            vectors = [item["embedding"] for item in data]
        else:
            response = http_json(
                "https://api.cohere.com/v2/embed",
                method="POST",
                headers={"Authorization": f"Bearer {self.api_key}"},
                body={
                    "model": self.model,
                    "texts": texts,
                    "input_type": "search_document",
                    "output_dimension": self.dimension,
                    "embedding_types": ["float"],
                },
            )
            vectors = response.get("embeddings", {}).get("float", [])

        if len(vectors) != len(texts):
            raise RuntimeError(f"Embedding API returned {len(vectors)} vectors for {len(texts)} texts")
        for index, vector in enumerate(vectors):
            if len(vector) != self.dimension:
                raise RuntimeError(
                    f"Embedding {index} has dimension {len(vector)}; expected {self.dimension}. "
                    "Check EMBEDDING_MODEL and output dimension support."
                )
        return vectors


class QdrantREST:
    def __init__(self, base_url: str, api_key: str):
        self.base_url = base_url.rstrip("/")
        self.headers = {"api-key": api_key} if api_key else {}

    def request(self, path: str, *, method: str = "GET", body: dict[str, Any] | None = None) -> dict[str, Any]:
        return http_json(f"{self.base_url}{path}", method=method, body=body, headers=self.headers)

    def collection(self, name: str) -> dict[str, Any]:
        return self.request(f"/collections/{quote(name, safe='')}").get("result", {})


def validate_target(qdrant: QdrantREST, name: str, vector_name: str, dimension: int) -> None:
    info = qdrant.collection(name)
    vectors = info.get("config", {}).get("params", {}).get("vectors")
    if vectors is None:
        raise RuntimeError(f"Could not read vector configuration for target collection {name!r}")
    if isinstance(vectors, dict) and "size" not in vectors:
        if vector_name not in vectors:
            raise RuntimeError(
                f"Target collection has named vectors {list(vectors)}; expected vector name {vector_name!r}. "
                "Set QDRANT_TARGET_VECTOR_NAME to match the collection."
            )
        config = vectors[vector_name]
    else:
        if vector_name:
            raise RuntimeError("Target collection uses an unnamed vector; set QDRANT_TARGET_VECTOR_NAME empty")
        config = vectors
    actual = config.get("size")
    if actual != dimension:
        raise RuntimeError(f"Target collection vector dimension is {actual}; expected {dimension}")
    distance = str(config.get("distance", "")).lower()
    if distance and distance != "cosine":
        raise RuntimeError(f"Target collection distance is {distance!r}; expected cosine")


def build_embedding_text(payload: dict[str, Any]) -> str:
    chunks = []
    for field in TEXT_FIELDS:
        value = payload.get(field)
        if isinstance(value, (str, int, float)) and str(value).strip():
            chunks.append(f"{field.replace('_', ' ').title()}: {str(value).strip()}")
    if not chunks:
        raise RuntimeError("A source point has no text in payload fields: " + ", ".join(TEXT_FIELDS))
    return ". ".join(chunks)


def target_point(point: dict[str, Any], vector_name: str, vector: list[float]) -> dict[str, Any]:
    result = {"id": point["id"], "payload": point.get("payload") or {}}
    result["vector"] = {vector_name: vector} if vector_name else vector
    return result


def main() -> int:
    load_dotenv()
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--provider", choices=("google", "openai", "cohere"), default=os.getenv("EMBEDDING_PROVIDER", "google"))
    parser.add_argument("--dimension", type=int, default=int(os.getenv("QDRANT_TARGET_VECTOR_SIZE", str(DEFAULT_DIMENSION))))
    parser.add_argument("--page-size", type=int, default=45)
    parser.add_argument("--dry-run", action="store_true", help="validate configuration but do not call embedding or upsert APIs")
    parser.add_argument("--reset-checkpoint", action="store_true", help="start over from the first source point")
    args = parser.parse_args()

    qdrant_key = os.getenv("QDRANT_API_KEY", "").strip()
    qdrant_url = os.getenv("QDRANT_URL", "").strip()
    if not qdrant_url:
        host = os.getenv("QDRANT_HOST", "").strip()
        if host:
            parsed_host = urlparse(host if "://" in host else f"//{host}")
            scheme = parsed_host.scheme or ("https" if qdrant_key else "http")
            hostname = parsed_host.hostname or host
            explicit_port = parsed_host.port
            configured_port = os.getenv("QDRANT_REST_PORT", "").strip()
            if not configured_port:
                grpc_port = os.getenv("QDRANT_PORT", "").strip()
                configured_port = "6333" if grpc_port == "6334" else (grpc_port or "6333")
            port = explicit_port or int(configured_port)
            default_port = 443 if scheme == "https" else 80
            qdrant_url = f"{scheme}://{hostname}" + (f":{port}" if port != default_port else "")
    if not qdrant_url:
        raise RuntimeError("Set QDRANT_URL, or configure QDRANT_HOST and QDRANT_API_KEY")
    source = os.getenv("QDRANT_SOURCE_COLLECTION", DEFAULT_SOURCE)
    target = os.getenv("QDRANT_TARGET_COLLECTION", DEFAULT_TARGET)
    vector_name = os.getenv("QDRANT_TARGET_VECTOR_NAME", DEFAULT_VECTOR_NAME)
    checkpoint = Path(os.getenv("QDRANT_MIGRATION_CHECKPOINT", ".qdrant-reembed-migration.json"))
    if source == target:
        raise RuntimeError("Source and target collection names must differ")
    if args.dimension <= 0 or not 1 <= args.page_size <= 256:
        raise RuntimeError("Dimension must be positive and page size must be between 1 and 256")

    qdrant = QdrantREST(qdrant_url, qdrant_key)
    qdrant.collection(source)  # fail early if the source collection is missing or inaccessible
    validate_target(qdrant, target, vector_name, args.dimension)
    embedding = EmbeddingClient(args.provider, args.dimension)

    print(f"Source: {source}; target: {target}; target vector: {vector_name or '(unnamed)'}")
    print(f"Embedding API: {args.provider}/{embedding.model}; dimension: {args.dimension}")
    if args.dry_run:
        print("Dry run passed: both collections and credentials are configured. No points were changed.")
        return 0

    state: dict[str, Any] = {}
    if checkpoint.exists() and not args.reset_checkpoint:
        state = json.loads(checkpoint.read_text(encoding="utf-8"))
        if state.get("source") != source or state.get("target") != target:
            raise RuntimeError(f"Checkpoint {checkpoint} belongs to a different migration; use --reset-checkpoint")
    elif args.reset_checkpoint and checkpoint.exists():
        checkpoint.unlink()
    if state.get("complete"):
        print(f"Migration was already marked complete by {checkpoint}.")
        return 0

    offset = state.get("offset")
    migrated = int(state.get("migrated", 0))
    while True:
        scroll_body: dict[str, Any] = {"limit": args.page_size, "with_payload": True, "with_vector": False}
        if offset is not None:
            scroll_body["offset"] = offset
        page = qdrant.request(
            f"/collections/{quote(source, safe='')}/points/scroll",
            method="POST",
            body=scroll_body,
        ).get("result", {})
        points = page.get("points", [])
        if not points:
            checkpoint.write_text(
                json.dumps({"source": source, "target": target, "migrated": migrated, "complete": True}, indent=2) + "\n",
                encoding="utf-8",
            )
            print(f"Migration complete. Re-embedded and copied {migrated} points.")
            return 0

        texts = [build_embedding_text(point.get("payload") or {}) for point in points]
        vectors = embedding.embed(texts)
        upsert_body = {"points": [target_point(point, vector_name, vector) for point, vector in zip(points, vectors)]}
        qdrant.request(
            f"/collections/{quote(target, safe='')}/points?wait=true",
            method="PUT",
            body=upsert_body,
        )

        migrated += len(points)
        offset = page.get("next_page_offset")
        checkpoint.write_text(
            json.dumps({"source": source, "target": target, "offset": offset, "migrated": migrated, "complete": False}, indent=2) + "\n",
            encoding="utf-8",
        )
        print(f"Migrated {migrated} points...", flush=True)
        if offset is None:
            checkpoint.write_text(
                json.dumps({"source": source, "target": target, "migrated": migrated, "complete": True}, indent=2) + "\n",
                encoding="utf-8",
            )
            print(f"Migration complete. Re-embedded and copied {migrated} points.")
            return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except KeyboardInterrupt:
        print("Interrupted. Re-run the same command to resume from the checkpoint.", file=sys.stderr)
        raise SystemExit(130)
    except Exception as exc:
        print(f"Migration failed: {exc}", file=sys.stderr)
        raise SystemExit(1)
