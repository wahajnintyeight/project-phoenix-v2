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
