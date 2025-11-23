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

# Install runtime dependencies
RUN apk --no-cache add \
    ca-certificates \
    ffmpeg \
    python3 \
    py3-pip \
    nodejs \
    curl

# Install yt-dlp
RUN pip3 install --break-system-packages -U yt-dlp

WORKDIR /app

COPY --from=builder /build/main .

# Copy runtime configs
COPY --from=builder /build/internal/service-configs ./internal/service-configs
COPY --from=builder /build/.env ./.env

EXPOSE 8885

CMD ["./main", "--service-name", "sse-service", "--port", "8885"]