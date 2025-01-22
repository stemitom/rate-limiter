#!/bin/bash
set -e  # Exit on any error

# Check if Redis is already running
if docker ps | grep -q test-redis; then
    echo "Stopping existing Redis container..."
    docker stop test-redis
    docker rm test-redis
fi

echo "Starting Redis..."
docker run -d --name test-redis -p 6379:6379 redis
sleep 5

echo "Running unit tests..."
go test ./internal/limiter -v || { echo "Unit tests failed"; exit 1; }

echo "Running integration tests..."
go test ./internal/limiter -tags=integration -v || { echo "Integration tests failed"; exit 1; }

echo "Running load test..."
go run cmd/loadtest/main.go || { echo "Load test failed"; exit 1; }

echo "Cleaning up..."
docker stop test-redis
docker rm test-redis

echo "All tests completed successfully" 
