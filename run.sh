#!/bin/bash

docker-compose up -d

echo "Waiting for services to start..."
sleep 10

echo "Services are ready!"
echo "Access Grafana at http://localhost:3000 (admin/admin)"
echo "Access Prometheus at http://localhost:9090"
echo "Rate Limiter is running at http://localhost:8081"
echo "Metrics endpoint is available at http://localhost:8083/metrics"
echo "To view logs, use: docker-compose logs -f"
