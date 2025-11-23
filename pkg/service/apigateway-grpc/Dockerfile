# Builder stage
FROM golang:1.24-alpine AS builder


WORKDIR /build

RUN apk add --no-cache git

COPY go.mod go.su* ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o main .

# Final stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /build/main .


EXPOSE 8886

# Healthcheck using TCP check since gRPC health probe requires server implementation
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD nc -z localhost 8886 || exit 1

CMD ["./main", "--service-name", "api-gateway-grpc", "--port", "8886"]