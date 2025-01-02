To build docker separately:

cd to project-phoenix-v2
docker build -t ppv2-<service-name> -f pkg/service/<service>/Dockerfile .

docker run -p <port>:<port> ppv2-<service-name> -d --name ppv2-<service-name> --restart always

# API Gateway Service

To build and run the API Gateway service:

docker build -t ppv2-apigateway -f pkg/service/apigateway/Dockerfile .
docker run -p 8881:8881 ppv2-apigateway -d --name ppv2-apigateway --restart always

# API Gateway GRPC Service

docker build -t ppv2-api-gateway-grpc -f pkg/service/apigateway-grpc/Dockerfile .
docker run -p 8886:8886 ppv2-api-gateway-grpc -d --name ppv2-apigateway-grpc --restart always



