# Builder stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy the go mod and sum files to download dependencies
COPY go.mod .
COPY go.sum .
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o main .
# Final stage
FROM alpine:latest  
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the pre-built binary file from the previous stage
COPY --from=builder /build .

EXPOSE 8887

# Command to run the executable
CMD ["./main","--service-name", "worker-service", "--port", "8887"]
