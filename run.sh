#!/bin/bash

docker run --name redis -d -p 6379:6379 redis

PORT=8081 go run cmd/ratelimiter/main.go &
PORT=8082 go run cmd/ratelimiter/main.go &

go run cmd/loadbalancer/main.go
