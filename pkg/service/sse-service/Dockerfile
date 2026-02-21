# Builder stage
FROM golang:1.24-alpine AS builder


WORKDIR /build

RUN apk add --no-cache git

# Use vendored modules from CI for deterministic, offline builds
COPY go.mod go.sum ./
COPY vendor ./vendor

COPY . .

ENV GOFLAGS="-mod=vendor"

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o main .

# Final stage
FROM alpine:3.19

# Install runtime dependencies (removed ffmpeg - no longer needed for direct streaming)
RUN apk --no-cache add \
    ca-certificates \
    aria2 \
    python3 \
    py3-pip \
    deno \
    ffmpeg \
    nodejs \
    curl

# Install latest yt-dlp binary (faster updates than distro/pip)
RUN curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o /usr/local/bin/yt-dlp && \
    chmod a+rx /usr/local/bin/yt-dlp

# Pre-cache JavaScript challenge solvers for YouTube signature decryption
RUN /usr/local/bin/yt-dlp --version && \
    /usr/local/bin/yt-dlp --remote-components ejs:github --version || true

# Install yt-dlp and crypto deps for python module usage
RUN pip3 install --break-system-packages --upgrade yt-dlp pycryptodome bgutil-ytdlp-pot-provider

WORKDIR /app

# Explicitly point app to yt-dlp location
ENV YT_DLP_BIN="/usr/local/bin/yt-dlp"

COPY --from=builder /build/main .

# Copy runtime configs
COPY --from=builder /build/internal/service-configs ./internal/service-configs


EXPOSE 8885

CMD ["./main", "--service-name", "sse-service", "--port", "8885"]