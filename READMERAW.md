To build docker separately:

cd to project-phoenix-v2
docker build -t ppv2-<service-name> -f pkg/service/<service>/Dockerfile .

docker run --name ppv2-<service-name> --restart always -p <port>:<port> ppv2-<service-name>

# API Gateway Service

To build and run the API Gateway service:

docker build -t ppv2-apigateway -f pkg/service/apigateway/Dockerfile .
docker run --name ppv2-api-gateway --restart always -p 8881:8881 ppv2-api-gateway

# API Gateway GRPC Service

docker build -t ppv2-api-gateway-grpc -f pkg/service/apigateway-grpc/Dockerfile .
docker run --name ppv2-api-gateway-grpc --restart always -p 8886:8886 ppv2-api-gateway-grpc



# Quick Build and Deploy


API Gateway
git pull && docker stop ppv2-api-gateway && docker rm ppv2-api-gateway && docker rmi ppv2-api-gateway && cd  /var/www/project-phoenix-v2 &&  docker build -t ppv2-api-gateway -f pkg/service/apigateway/Dockerfile . && docker run --name ppv2-api-gateway -d --restart always -p 8881:8881 ppv2-api-gateway

Socket Service
git pull && docker stop ppv2-socket-service && docker rm ppv2-socket-service && docker rmi ppv2-socket-service &&   docker build -t ppv2-socket-service -f pkg/service/socketservice/Dockerfile . && docker run --name ppv2-socket-service -d --restart always -p 8884:8884 ppv2-socket-service

SSE Service
git pull && docker stop ppv2-sse-service && docker rm ppv2-sse-service && docker rmi ppv2-sse-service &&  docker build -t ppv2-sse-service -f pkg/service/sse-service/Dockerfile . && docker run --name ppv2-sse-service -d --restart always -p 8882:8885 ppv2-sse-service

Worker Service
git pull && docker stop ppv2-worker-service && docker rm ppv2-worker-service && docker rmi ppv2-worker-service && docker build -t ppv2-worker-service -f pkg/service/worker-service/Dockerfile . && docker run --name ppv2-worker-service -d --restart always -p 8887:8887 ppv2-worker-service

Scraper Service
git pull && docker stop ppv2-scraper-service && docker rm ppv2-scraper-service && docker rmi ppv2-scraper-service && docker build -t ppv2-scraper-service -f pkg/service/scraper-service/Dockerfile . && docker run --name ppv2-scraper-service -d --restart always -p 8888:8888 ppv2-scraper-service

# Docker Compose Commands (Recommended)

Restart single service with new env vars (no rebuild):
docker compose -p ppv2 up -d --force-recreate <service-name>

Examples:
docker compose -p ppv2 up -d --force-recreate scraper-service
docker compose -p ppv2 up -d --force-recreate worker-service
docker compose -p ppv2 up -d --force-recreate api-gateway

Pull latest images and restart:
docker compose -p ppv2 pull <service-name> && docker compose -p ppv2 up -d --no-deps <service-name>

Restart all services:
docker compose -p ppv2 restart

View logs:
docker compose -p ppv2 logs <service-name> -f --tail=100

Check status:
docker compose -p ppv2 ps

Stop all services:
docker compose -p ppv2 down

Start all services:
docker compose -p ppv2 up -d
