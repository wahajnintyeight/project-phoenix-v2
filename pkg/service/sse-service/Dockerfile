# Builder stage
FROM golang:latest AS builder


WORKDIR /build

# Copy the go mod and sum files to download dependencies
COPY go.mod .
COPY go.sum .
RUN go mod download

# Copy the source code into the container
COPY . .

#COPY internal/service-configs/ /app/internal/service-configs/
#RUN ls -R /app/internal/service-configs/

# Build the application
#RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .
#RUN CGO_ENABLED=0 GOOS=linux go build -o /main
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o main .
FROM scratch

# Final stage
FROM alpine:latest  
RUN apk --no-cache add ca-certificates ffmpeg python3 py3-pip
RUN pip3 install --break-system-packages yt-dlp

WORKDIR /app

# Copy the pre-built binary file from the previous stage
COPY --from=builder /build .

EXPOSE 8885

# Command to run the executable
# main.go --service-name api-gateway --port 8889
CMD ["./main", "--service-name", "sse-service", "--port", "8885"]