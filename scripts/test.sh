#!/bin/bash

docker run -d --name test-redis -p 6379:6379 redis
sleep 5

# Run unit tests
go test ./internal/limiter -v

# Run integration tests
# go test ./internal/limiter -tags=integration -v

# Run load test
go run cmd/loadtest/main.go

# Cleanup
docker stop test-redis
docker rm test-redis 
