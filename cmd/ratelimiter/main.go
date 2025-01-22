package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/stemitom/rate-limiter/internal/limiter"
)

var (
	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"status"},
	)
	rateLimitHits = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "rate_limit_hits_total",
			Help: "Total number of rate limit hits.",
		},
	)
)

func init() {
	prometheus.MustRegister(requestsTotal)
	prometheus.MustRegister(rateLimitHits)
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return xrip
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func main() {
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	rateLimit, _ := strconv.Atoi(getEnv("RATE_LIMIT", "10"))
	windowSize, _ := time.ParseDuration(getEnv("WINDOW_SIZE", "1m"))
	port := getEnv("PORT", "8081")

	log.Printf("Connecting to Redis at %s", redisAddr)
	log.Printf("Rate limit: %d requests per %v", rateLimit, windowSize)

	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	defer rdb.Close()

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Println("Successfully connected to Redis")

	limiter := limiter.NewRateLimiter(rdb, rateLimit, windowSize)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		key := getClientIP(r)
		// log.Printf("Handling request from IP: %s", key)

		allowed, err := limiter.Allow(ctx, key)
		if err != nil {
			// log.Printf("Error checking rate limit: %v", err)
			requestsTotal.WithLabelValues("500").Inc()
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if !allowed {
			// log.Printf("Rate limit exceeded for IP: %s", key)
			requestsTotal.WithLabelValues("429").Inc()
			rateLimitHits.Inc()
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		// log.Printf("Request allowed for IP: %s", key)
		requestsTotal.WithLabelValues("200").Inc()
		fmt.Fprintln(w, "Request allowed")
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	http.Handle("/metrics", promhttp.Handler())

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Rate limiter service started on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
