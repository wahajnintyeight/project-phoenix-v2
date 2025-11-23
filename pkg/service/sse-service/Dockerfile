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


EXPOSE 8885

CMD ["./main", "--service-name", "sse-service", "--port", "8885"]