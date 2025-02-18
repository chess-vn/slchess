# Slchess

## Overview

This project is a backend server for a gaming platform, designed using AWS SAM, Golang, task, Docker, and Swagger OpenAPI. The server is designed to handle multiple concurrent chess matches, integrate matchmaking based on ELO ratings, and provide real-time updates to players via WebSockets. The architecture leverages AWS Fargate for serverless container management, providing scalability and flexibility.

## Technologies

- **Golang**: The backend is written in Go, providing a highly performant and concurrent platform for handling game logic and matchmaking.
- **AWS SAM (Serverless Application Model)**: Used for defining and deploying serverless applications.
- **task**: Task runner for automating common development tasks such as building, testing, and running Docker containers.
- **Docker**: Containers are used to package and deploy the application in a consistent environment.
- **Swagger OpenAPI**: API documentation and design system for defining and visualizing the application's WebSocket and RESTful APIs.

## Features

- **ELO-based Matchmaking**: Matches are created based on player ratings, with a maximum ELO difference of 100.
- **WebSocket Communication**: Real-time messaging for match invitations, notifications, and game updates.
- **Serverless Architecture**: Deployed using AWS Fargate with auto-scaling based on demand.
- **API Documentation**: Swagger OpenAPI is used to define and visualize the API endpoints, ensuring clear and consistent communication between frontend and backend.

## Project Structure

```
├── cmd/                  # Main application code
├── configs/              # Configuration files for game server
├── docs/                 # Project documentations
├── events/               # Events to test local lambda function
├── internal/
├── pkg/
├── taskfiles/            # Taskfiles for task
├── test/                 # Files for testing
├── Taskfile.yml          # task command definitions
├── compose.yml           # Docker compose file for game server
├── samconfig.yaml        # AWS SAM config file (auto-generated)
├── template.yaml         # AWS SAM template file to define AWS CloudFormation stack
├── server.dockerfile     # Dockerfile to build the game server image
```

## Setup

### Prerequisites

Before you begin, ensure you have the following installed:

- [Go](https://golang.org/doc/install)
- [AWS CLI](https://aws.amazon.com/cli/)
- [AWS SAM CLI](https://aws.amazon.com/serverless/sam/)
- [Docker](https://www.docker.com/products/docker-desktop)
- [task](https://taskfile.dev/)

### Run game server locally

```bash
task server:local
```

### Run game server in docker container

```bash
# Start
task server:up

# Stop
task server:down
```

### Deploy with AWS SAM

1. Build the application:

   ```bash
   task stack:build
   ```

2. Deploy the application:

   ```bash
   task stack:deploy
   ```

### Test functionalities

#### Matchmaking

1. Get test user credentials:

   ```bash
   task cognito:test-authenticate-users
   ```

2. Copy the tokens to HTTP Authorization header and send
3. Get the game server public ip address from HTTP response for playing

## API Documentation

The project uses Swagger OpenAPI to define the API. You can view and interact with the API documentation by navigating to:

- **WebSocket API**: View the WebSocket API definition and try out the connections.
- **REST API**: View and test the endpoints for other functionalities.

To run the Swagger UI locally, use the following command:

```bash
# Run HTTP API container
task doc:api

# Run WebSocket API container
task doc:awpi
```

## Contributing

We welcome contributions to this project!
