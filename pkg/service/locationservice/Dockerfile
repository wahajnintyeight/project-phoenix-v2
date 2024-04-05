# Builder stage
FROM golang:1.21.6 AS builder

WORKDIR /app

# Copy the go mod and sum files to download dependencies
COPY go.mod .
COPY go.sum .
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Final stage
FROM alpine:latest  
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the pre-built binary file from the previous stage
COPY --from=builder /app/main .

EXPOSE 8882

# Command to run the executable
# main.go --service-name api-gateway --port 8889
CMD ["./main","--service-name", "location-service", "--port", "8882"]