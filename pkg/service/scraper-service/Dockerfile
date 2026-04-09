# Builder stage
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Install git required for fetching dependencies
RUN apk add --no-cache git

# Copy module files and vendor directory prepared by CI
COPY go.mod go.sum ./
COPY vendor ./vendor

# Copy the source code
COPY . .

# Ensure vendored build (no network, no checksum resolution)
ENV GOFLAGS="-mod=vendor"

# Build the application
# -ldflags="-w -s" reduces binary size by stripping debug information
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o main .

# Final stage
FROM alpine:latest

# Install CA certificates and wget for healthcheck
RUN apk --no-cache add ca-certificates wget

WORKDIR /app

# Copy only the compiled binary from builder
COPY --from=builder /build/main .

# Copy runtime configs
COPY --from=builder /build/internal/service-configs ./internal/service-configs

EXPOSE 8888

CMD ["./main", "--service-name", "scraper-service", "--port", "8888"]
