# Builder stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Install git required for fetching dependencies
RUN apk add --no-cache git

# Copy go mod and sum files first to leverage Docker cache
COPY go.mod go.su* ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application
# -ldflags="-w -s" reduces binary size by stripping debug information
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o main .

# Final stage
FROM alpine:3.19

# Install CA certificates and curl for healthcheck
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy only the compiled binary from builder
COPY --from=builder /build/main .


EXPOSE 8881


CMD ["./main", "--service-name", "api-gateway", "--port", "8881"]