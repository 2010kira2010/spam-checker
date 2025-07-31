# Build stage for React frontend
FROM node:18-alpine AS frontend-builder

WORKDIR /app

# Copy package files first for better caching
COPY frontend/package.json frontend/package-lock.json* ./

# Install ALL dependencies (including devDependencies needed for build)
RUN npm ci

# Copy frontend source code
COPY frontend/ .

# Build frontend application
RUN npm run build

# Build stage for Go backend
FROM golang:1.24-alpine AS backend-builder

# Install build dependencies including swag for documentation
RUN apk add --no-cache git gcc musl-dev && \
    go install github.com/swaggo/swag/cmd/swag@latest

WORKDIR /app

# Copy go mod files for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download && go mod tidy

# Copy the entire project
COPY . .

# Generate swagger documentation
RUN swag init -g ./cmd/main.go -o ./docs --parseDependency --parseInternal

# Build the Go application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/main.go

# Final production stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add \
    ca-certificates \
    tesseract-ocr \
    tesseract-ocr-data-rus \
    tesseract-ocr-data-eng \
    && rm -rf /var/cache/apk/*

WORKDIR /app

# Copy backend binary from builder
COPY --from=backend-builder /app/main .

# Copy frontend build files to static directory
COPY --from=frontend-builder /app/build ./static

# Copy swagger docs
COPY --from=backend-builder /app/docs ./docs

# Create necessary directories
RUN mkdir -p /app/screenshots /app/logs && \
    chmod -R 755 /app

# Make binary executable
RUN chmod +x /app/main

# Expose the application port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
CMD ["./main"]