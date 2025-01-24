# Distributed Rate Limiter

A distributed rate limiter service built with Go, featuring Redis-backed storage and load balancing capabilities. The service also includes monitoring with Prometheus and Grafana.

## Features

- Redis-backed sliding window rate limiting algorithm
- Distributed architecture with load balancing
- Prometheus metrics and Grafana dashboards
- Docker containerization
- Comprehensive test suite (unit, integration, and load tests)
- Configurable rate limits and time windows
- Health checks and automatic backend failover

## Architecture

<img src="docs/d2.svg" width="300" alt="Architecture Diagram">

The system consists of several components:

- **Rate Limiter Service**: Implements the core rate limiting logic using a sliding window algorithm
- **Load Balancer**: Distributes traffic across multiple rate limiter instances
- **Redis**: Stores rate limiting data and enables distributed coordination
- **Prometheus**: Collects and stores metrics
- **Grafana**: Visualizes metrics and provides monitoring dashboards

## Prerequisites

- Docker and Docker Compose
- Go 1.22 or later (for local development)
- Redis (automatically handled by Docker Compose)

## Quick Start

1. Clone the repository:
   ```bash
   git clone https://github.com/stemitom/rate-limiter.git
   cd rate-limiter
   ```

2. Start the services:
   ```bash
   ./run.sh
   ```

This will start:
- Two rate limiter instances (`::8081`, `::8082`)
- Load balancer (`::8080`)
- Redis (`::6379`)
- Prometheus (`::9090`)
- Grafana (`::3000`)

## Configuration

The following environment variables can be configured:

### Rate Limiter Service
- `PORT`: Service port (default: 8081)
- `REDIS_ADDR`: Redis address (default: localhost:6379)
- `RATE_LIMIT`: Requests per window (default: 10)
- `WINDOW_SIZE`: Time window duration (default: 1s)

### Load Balancer
- `BACKEND_1_URL`: First backend URL
- `BACKEND_2_URL`: Second backend URL
- `BACKEND_1_WEIGHT`: Traffic weight for first backend
- `BACKEND_2_WEIGHT`: Traffic weight for second backend

## Testing

Run the test suite:
```bash
./scripts/test.sh
```

This will execute:
- Unit tests
- Integration tests
- Load tests

## Load Testing

The project includes a load testing tool that can be used to benchmark the rate limiter:

```bash
go run cmd/loadtest/main.go -rps 100 -duration 10s -url http://localhost:8080
```

Parameters:
- `-rps`: Requests per second
- `-duration`: Test duration
- `-url`: Target URL

## Monitoring

- Grafana Dashboard: http://localhost:3000 (default credentials: admin/admin)
- Prometheus: http://localhost:9090
- Metrics endpoints:
  - Rate Limiter: http://localhost:8081/metrics
  - Load Balancer: http://localhost:8080/metrics

## API Endpoints

- `GET /`: Main endpoint for rate-limited requests
- `GET /health`: Health check endpoint
- `GET /metrics`: Prometheus metrics endpoint

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.