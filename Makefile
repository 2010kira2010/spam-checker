.PHONY: help build run dev clean test swagger frontend docker-build docker-up docker-down install

# Default target
help:
	@echo "Available commands:"
	@echo "  make install       - Install all dependencies"
	@echo "  make build         - Build the application"
	@echo "  make run           - Run the application"
	@echo "  make dev           - Run in development mode with hot reload"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make test          - Run tests"
	@echo "  make swagger       - Generate Swagger documentation"
	@echo "  make frontend      - Build frontend"
	@echo "  make docker-build  - Build Docker image"
	@echo "  make docker-up     - Start all services with Docker Compose"
	@echo "  make docker-down   - Stop all services"

# Install dependencies
install:
	@echo "Installing backend dependencies..."
	go mod download
	go mod tidy
	@echo "Installing frontend dependencies..."
	cd frontend && npm install
	@echo "Installing development tools..."
	go install github.com/cosmtrek/air@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	@echo "Generating initial swagger docs..."
	@make swagger

# Build backend
build-backend:
	@echo "Building backend..."
	go build -o bin/spamchecker ./cmd/main.go

# Build frontend
build-frontend:
	@echo "Building frontend..."
	cd frontend && npm install && npm run build
	rm -rf static
	cp -r frontend/build static

# Build everything
build: swagger build-frontend build-backend

# Run the application
run: build
	./bin/spamchecker

# Development mode with hot reload
dev:
	@echo "Building frontend for development..."
	cd frontend && npm install && npm run build
	rm -rf static
	cp -r frontend/build static
	@echo "Starting development server..."
	air

# Development mode with frontend watch
dev-full:
	@echo "Starting full development mode..."
	@echo "Start frontend dev server: cd frontend && npm start"
	@echo "Backend will use proxy to frontend dev server"
	air

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/ tmp/ static/ frontend/build/ frontend/node_modules/
	go clean

# Run tests
test:
	@echo "Running backend tests..."
	go test -v ./...
	@echo "Running frontend tests..."
	cd frontend && npm test -- --watchAll=false

# Generate Swagger documentation
swagger:
	@echo "Generating Swagger documentation..."
	swag init -g ./cmd/main.go -o ./docs --parseInternal --generatedTime

# Docker commands
docker-build: swagger
	@echo "Building Docker image..."
	docker build -t spamchecker:latest .

docker-build-dev:
	@echo "Building development Docker image..."
	docker build -f Dockerfile.dev -t spamchecker:dev .

docker-up:
	@echo "Starting production services..."
	docker-compose up -d

docker-up-dev:
	@echo "Starting development services..."
	docker-compose -f docker-compose.dev.yml up -d

docker-down:
	@echo "Stopping services..."
	docker-compose down

docker-down-dev:
	@echo "Stopping development services..."
	docker-compose -f docker-compose.dev.yml down

docker-logs:
	docker-compose logs -f app

docker-rebuild:
	@echo "Rebuilding and starting services..."
	docker-compose down
	docker-compose build --no-cache
	docker-compose up -d

# Database migrations
migrate-up:
	@echo "Running migrations..."
	go run cmd/migrate/main.go up

migrate-down:
	@echo "Rolling back migrations..."
	go run cmd/migrate/main.go down

# Format code
fmt:
	@echo "Formatting backend code..."
	go fmt ./...
	gofmt -s -w .
	@echo "Formatting frontend code..."
	cd frontend && npm run format 2>/dev/null || echo "No format script defined"

# Lint code
lint:
	@echo "Linting backend code..."
	golangci-lint run 2>/dev/null || echo "golangci-lint not installed"
	@echo "Linting frontend code..."
	cd frontend && npm run lint 2>/dev/null || echo "No lint script defined"

# Full development setup
setup: install
	@echo "Setting up development environment..."
	cp .env.example .env
	@echo "Setup complete! Edit .env file and run 'make dev' to start development."

# Production build
prod-build:
	@echo "Creating production build..."
	docker build -t spamchecker:prod .
	@echo "Production image built: spamchecker:prod"

# Quick start for development
quick-start: setup
	@echo "Starting SpamChecker in development mode..."
	make dev