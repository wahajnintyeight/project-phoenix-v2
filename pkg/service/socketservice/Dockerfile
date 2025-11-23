# Builder stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

RUN apk add --no-cache git

# Use vendor prepared by CI to avoid go.sum/network issues
COPY go.mod go.sum ./
COPY vendor ./vendor

# Copy the source code
COPY . .

# Enforce vendored build
ENV GOFLAGS="-mod=vendor"

# Build static binary for minimal final image
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o main .

# Final stage
FROM alpine:3.19
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy only the compiled binary
COPY --from=builder /build/main .

EXPOSE 8884

CMD ["./main", "--service-name", "socket-service", "--port", "8884"]