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




