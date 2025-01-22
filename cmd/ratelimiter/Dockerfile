# Use the official Go image as the base image
FROM golang:1.22-alpine AS builder

# Set the working directory
WORKDIR /app

# Copy the Go module files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o ratelimiter ./cmd/ratelimiter

# Use a minimal Alpine image for the final stage
FROM alpine:latest

# Install CA certificates (needed for HTTPS requests)
RUN apk --no-cache add ca-certificates

# Set the working directory
WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /app/ratelimiter .

# Expose the port
EXPOSE 8081

# Run the application
CMD ["./ratelimiter"]
