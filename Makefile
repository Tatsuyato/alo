.PHONY: build run test clean lint fmt

# Default target
default: run

# Build the binary
build:
	cd src && go build -o ../alo

# Run the app (dev mode with hot reload on file change you can use air)
run:
	go run ./src

# Run with environment variables pre-set (local dev)
run-dev:
	PORT=8081 DB_PATH=./dev.db API_KEY_ADMIN=dev-admin API_KEY_DEPLOYER=dev-deployer JWT_SECRET=dev-secret go run ./src

# Run production build
run-prod: build
	./alo

# Format all Go files
fmt:
	gofmt -w ./src

# Lint code
lint:
	go vet ./src/...

# Run tests
test:
	go test -v ./src/...

# Clean build artifacts
clean:
	rm -f ./alo
	go clean

# Install dependencies
deps:
	go mod download
	go mod verify

# Tidy up mod files
tidy:
	go mod tidy

# Docker build
docker-build:
	docker build -t alo-backend:latest .

# Docker run
docker-run:
	docker run -p 8080:8080 alo-backend:latest

# Full rebuild
rebuild: clean tidy fmt build

help:
	@echo "Available targets:"
	@echo "  make build       - Build binary to ./alo"
	@echo "  make run         - Run in dev mode (hot reload)"
	@echo "  make run-dev     - Run with dev environment variables"
	@echo "  make run-prod    - Build and run production binary"
	@echo "  make fmt         - Format Go code"
	@echo "  make lint        - Lint code with go vet"
	@echo "  make test        - Run tests"
	@echo "  make clean       - Remove build artifacts"
	@echo "  make deps        - Download and verify dependencies"
	@echo "  make tidy        - Tidy go.mod"
	@echo "  make rebuild     - Clean, tidy, fmt, and build"
	@echo "  make docker-build - Build Docker image"
	@echo "  make docker-run  - Run Docker container"
