# Chirpy - Go Server Practice Project

A practice project for learning Go server development through Boot.dev's curriculum. Chirpy is a simple HTTP server implementation designed to demonstrate core concepts in web development using Go.

## Overview

This project serves as a learning resource for building HTTP servers in Go. It covers fundamental concepts including routing, request handling, JSON serialization, and basic server architecture.

## Features

- HTTP server built with Go's standard library
- RESTful API endpoints
- JSON request and response handling
- Clean project structure and code organization
- Practical examples of Go web development patterns

## Project Structure

```
.
├── README.md           # Project documentation
├── main.go            # Entry point for the application
├── handlers/          # HTTP request handlers
├── models/            # Data structures and models
└── config/            # Configuration files
```

## Getting Started

### Prerequisites

- Go 1.16 or higher
- Git

### Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/Bravnar/bootdev_go_server_practice_Chirpy.git
   cd bootdev_go_server_practice_Chirpy
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Run the server:
   ```bash
   go run main.go
   ```

The server will start on the default port (typically localhost:8080).

## Usage

### API Endpoints

The server provides the following endpoints:

- `GET /` - Health check endpoint
- `GET /api/healthz` - Server health status
- `POST /api/chirps` - Create a new chirp
- `GET /api/chirps` - Retrieve all chirps

### Example Request

```bash
curl http://localhost:8080/api/healthz
```

## Development

### Running Tests

To run the test suite:

```bash
go test ./...
```

### Building

To build the executable:

```bash
go build -o chirpy
```

## Learning Objectives

This project demonstrates:

- Setting up an HTTP server in Go
- Handling HTTP routes and methods
- Processing JSON payloads
- Error handling and validation
- Testing Go applications
- Project structure and organization

## Resources

- [Official Go Documentation](https://golang.org/doc)
- [Boot.dev Course](https://boot.dev)
- [HTTP Server in Go](https://pkg.go.dev/net/http)

## License

This project is part of Boot.dev's educational curriculum.

## Author

Bravnar
