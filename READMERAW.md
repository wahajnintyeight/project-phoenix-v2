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
git pull && docker stop ppv2-api-gateway && docker rm ppv2-api-gateway && docker rmi ppv2-apigateway && cd  /var/www/project-phoenix-v2 &&  docker build -t ppv2-apigateway -f pkg/service/apigateway/Dockerfile . && docker run --name ppv2-api-gateway -d --restart always -p 8881:8881 ppv2-api-gateway

Socket Service
git pull && docker stop ppv2-socket-service && docker rm ppv2-socket-service && docker rmi ppv2-socket-service &&   docker build -t ppv2-socket-service -f pkg/service/socketservice/Dockerfile . && docker run --name ppv2-socket-service -d --restart always -p 8884:8884 ppv2-socket-service

SSE Service
git pull && docker stop ppv2-sse-service && docker rm ppv2-sse-service && docker rmi ppv2-sse-service &&  docker build -t ppv2-sse-service -f pkg/service/sse-service/Dockerfile . && docker run --name ppv2-sse-service -d --restart always -p 8882:8885 ppv2-sse-service
