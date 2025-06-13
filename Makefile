.PHONY: all build test test-unit test-integration clean lint run docker-build docker-run

# Variables
BINARY_NAME=lightfile6-insights-gateway
MAIN_PATH=./cmd/gateway
DOCKER_IMAGE=lightfile6-insights-gateway:latest

# Default target
all: test build

# Build the binary
build:
	go build -o $(BINARY_NAME) $(MAIN_PATH)

# Run all tests
test: test-unit test-integration

# Run unit tests
test-unit:
	go test -v -race -coverprofile=coverage.out ./...

# Run integration tests
test-integration:
	cd test/integration && go test -v -tags=integration -timeout=10m

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out
	rm -rf dist/

# Run linting
lint:
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping..."; \
	fi

# Run the application locally
run: build
	./$(BINARY_NAME) -p 8080 -c config.yml

# Build Docker image
docker-build:
	docker build -t $(DOCKER_IMAGE) .

# Run with docker-compose
docker-run:
	docker-compose up

# Run tests in Docker
docker-test:
	docker-compose -f docker-compose.test.yml up --build --abort-on-container-exit

# Format code
fmt:
	go fmt ./...

# Download dependencies
deps:
	go mod download
	go mod tidy

# Generate test coverage report
coverage: test-unit
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Check for security vulnerabilities
security:
	@if command -v gosec > /dev/null; then \
		gosec ./...; \
	else \
		echo "gosec not installed, skipping..."; \
	fi

# Run go vet
vet:
	go vet ./...

# Quick quality check
check: fmt vet lint

# Release with goreleaser (for testing)
release-snapshot:
	@if command -v goreleaser > /dev/null; then \
		goreleaser release --snapshot --clean; \
	else \
		echo "goreleaser not installed, skipping..."; \
	fi