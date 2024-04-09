<!DOCTYPE html>
<html lang="en">
<body>

<div class="container">
  <h1>Project Phoenix V2</h1>
  <p>Project Phoenix V2 is a comprehensive solution designed to streamline operations and enhance functionality for modern web applications. With a focus on scalability and performance, this project aims to provide a robust framework for building and deploying services efficiently.</p>

 <h2>About This Version</h2>
  <p>The V1 of this project was developed using Node.js, focusing on providing a solid foundation for web applications. In this new iteration, Project Phoenix V2, I've transitioned to using Go. This move aims to offer better stability and a significant increase in performance, addressing the evolving needs of modern web services and applications.</p>

  <h2>Features</h2>
  <ul>
    <li><strong>API Gateway Integration:</strong> Simplifies interaction between client applications and backend services through a unified entry point.</li>
    <li><strong>Scalable Architecture:</strong> Designed to support scaling operations, ensuring high availability and resilience.</li>
    <li><strong>Customizable Modules:</strong> Includes a set of modules that can be tailored to meet specific requirements, enhancing flexibility and efficiency.</li>
  </ul>

  <h2>Getting Started</h2>
  <p>These instructions will get you a copy of the project up and running on your local machine for development and testing purposes. See deployment for notes on how to deploy the project on a live system.</p>

  
<div class="container">
    <h1>Project Phoenix V2 Documentation</h1>
    <p>This document provides an overview of the LocationService component within Project Phoenix V2, including setup instructions, service architecture, and usage.</p>

  <section id="overview">
      <h2>Service Overview</h2>
      <p>The LocationService is designed to manage geolocation tracking within Project Phoenix. It subscribes to specific topics through a message broker (RabbitMQ) to handle start and stop tracking events.</p>
  </section>

  <section id="architecture">
      <h2>Architecture</h2>
      <p>The service is built on the Go-Micro framework, leveraging the power of microservices for scalable, distributed applications. RabbitMQ is used as the message broker for event-driven communication between services.</p>
  </section>

   <section id="api-gateway-service">
      <h2>API Gateway Service</h2>
      <p>The API Gateway Service acts as the entry point for all client requests to the backend services. It routes requests to the appropriate service, handles sessions, and provides an additional layer of security through middleware.</p>
  <section id="setup-api-gateway">
      <h2>Setting Up the API Gateway Service</h2>
      <p>Configure and run the API Gateway Service to manage incoming requests and route them to the correct backend services.</p>
      
  <h3>Configuration</h3>
  <p>Ensure RabbitMQ is running and accessible. Configure the .env file with the correct RabbitMQ connection parameters:</p>
  <code>RABBITMQ_HOST=localhost<br>
      RABBITMQ_PORT=5672<br>
      RABBITMQ_USERNAME=user<br>
      RABBITMQ_PASSWORD=pass
  </code>

  <h3>Running the Service</h3>
  <p>Navigate to the project directory and execute:</p>
  <code>go run main.go --service-name api-gateway --port 8981</code>
  </section>

  <section id="registering-routes">
      <h2>Registering Routes and Middleware</h2>
      <p>The API Gateway Service uses the Gorilla Mux router to define HTTP routes. Middleware can be applied to routes for session management, authentication, and logging.</p>

  <h3>Example Route Registration</h3>
  <code>api.router.HandleFunc("/api/path", handlerFunction).Methods("GET")</code>
  
  <p>Middleware is applied to routes to handle cross-cutting concerns:</p>
  <code>api.router.Use(authMiddleware)</code>
  </section>

  <section id="service-communication">
      <h2>Service Communication</h2>
      <p>The API Gateway communicates with backend services through RabbitMQ for message-based communication, facilitating asynchronous processing and decoupling of services.</p>
  </section>

  <section id="docker-commands-api-gateway">
      <h2>Docker Commands for API Gateway</h2>
      <p>To containerize the API Gateway Service, use the provided Docker commands to build images, run containers, and view logs.</p>
      
      <h3>Building Docker Images</h3>
      <code>docker build -t api-gateway .</code>

      <h3>Running the API Gateway Container</h3>
      <code>docker run -d -p 8981:8981 api-gateway</code>
  </section>    
  <h3>Key Features</h3>
  <ul>
      <li>Unified entry point for backend services, simplifying client interactions.</li>
      <li>Session management capabilities to maintain user state across requests.</li>
      <li>Modular middleware integration for authentication, logging, and request validation.</li>
      <li>Flexible routing to support various backend services dynamically.</li>
  </ul>
  </section>

  <section id="setup">
      <h2>Setup and Configuration</h2>
      <h3>Prerequisites</h3>
      <ul>
          <li>Go (1.x or later)</li>
          <li>Docker (for RabbitMQ container)</li>
          <li>RabbitMQ (standalone or via Docker)</li>
      </ul>

  <h3>Installation</h3>
      <p>Follow these steps to get the LocationService running:</p>
      <ol>
          <li>Clone the Project Phoenix repository: <code>git clone https://github.com/wahajnintyeight/project-phoenix-v2.git</code></li>
          <li>Navigate to the project directory: <code>cd project-phoenix-v2</code></li>
          <li>Build and run the service: <code>go run main.go --service-name location-service --port 8982</code></li>
      </ol>
  </section>

  
  <section id="service-usage">
      <h2>Service Usage</h2>
      <p>The LocationService subscribes to 'start-tracking' and 'stop-tracking' topics. It processes incoming messages to manage tracking operations for users or devices.</p>
  </section>

</div>

  <h3>Prerequisites</h3>
  <p>What things you need to install the software and how to install them:</p>
  <ul>
    <li>Go (version 1.x or later)</li>
  </ul>

  <h3>Installing</h3>
  <p>A step-by-step series of examples that tell you how to get a development environment running:</p>
  <ol>
    <li><strong>Clone the Repository</strong>
      <code>git clone https://github.com/wahajnintyeight/project-phoenix-v2.git</code>
    </li>
    <li><strong>Navigate to the Project Directory</strong>
      <code>cd project-phoenix-v2</code>
    </li>
    <li><strong>Run the Application</strong>
      <code>go run main.go --service-name api-gateway --port 8981</code>
    </li>
  </ol>

   <h2>Docker Commands</h2>
  <p>Below are some useful Docker commands for managing the containers associated with Project Phoenix V2.</p>

  <h3>Building and Running Containers</h3>
  <p>To build the Docker images and run the containers in detached mode, use the following command:</p>
  <code>docker compose up --build -d</code>

  <h3>Stopping All Containers</h3>
  <p>To stop all running containers:</p>
  <code>sudo docker stop $(sudo docker ps -a -q)</code>

  <h3>Removing All Containers</h3>
  <p>To remove all containers, ensuring you can rebuild and start fresh, use:</p>
  <code>sudo docker rm $(sudo docker ps -a -q)</code>

  <h3>Viewing Logs of a Container</h3>
  <p>To follow the logs of a specific container, which is useful for debugging and monitoring the application's output:</p>
  <code>sudo docker logs --follow containername</code>

  <h3>Other Useful Docker Commands</h3>
  <ul>
    <li><strong>List All Containers:</strong> <code>docker ps -a</code></li>
    <li><strong>List Running Containers:</strong> <code>docker ps</code></li>
    <li><strong>Access Container Shell:</strong> <code>docker exec -it containername /bin/sh</code></li>
    <li><strong>Build Docker Images:</strong> <code>docker build -t imagename .</code></li>
    <li><strong>Removing Docker Images:</strong> <code>docker rmi imagename</code></li>
    <li><strong>Viewing Docker Images:</strong> <code>docker images</code></li>
  </ul>

  <h2>Running the Tests</h2>
  <p>Explain how to run the automated tests for this system:</p>
  <code>go test ./...</code>

  <h2>Built With</h2>
  <ul>
    <li><a href="https://golang.org/">Go</a> - The programming language used</li>
    <li><a href="https://www.docker.com/">Docker</a> - Containerization platform (optional)</li>
  </ul>
</div>

</body>
</html>
