package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
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
	requestLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"status"},
	)
	redisLatency = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "redis_operation_duration_seconds",
			Help:    "Redis operation latency in seconds.",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
	)
)

func init() {
	prometheus.MustRegister(requestsTotal)
	prometheus.MustRegister(rateLimitHits)
	prometheus.MustRegister(requestLatency)
	prometheus.MustRegister(redisLatency)
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
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

func setRateLimitHeaders(w http.ResponseWriter, result limiter.Result) {
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(result.Limit))
	w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(result.Remaining, 10))
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))
}

type HealthResponse struct {
	Status string `json:"status"`
	Redis  string `json:"redis"`
}

func main() {
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	rateLimit := getEnvInt("RATE_LIMIT", 10)
	windowSize := getEnvDuration("WINDOW_SIZE", time.Second)
	port := getEnv("PORT", "8081")

	redisPoolSize := getEnvInt("REDIS_POOL_SIZE", 100)
	redisMinIdleConns := getEnvInt("REDIS_MIN_IDLE_CONNS", 10)
	redisDialTimeout := getEnvDuration("REDIS_DIAL_TIMEOUT", 5*time.Second)
	redisReadTimeout := getEnvDuration("REDIS_READ_TIMEOUT", 3*time.Second)
	redisWriteTimeout := getEnvDuration("REDIS_WRITE_TIMEOUT", 3*time.Second)

	log.Printf("Connecting to Redis at %s", redisAddr)
	log.Printf("Rate limit: %d requests per %v", rateLimit, windowSize)

	rdb := redis.NewClient(&redis.Options{
		Addr:            redisAddr,
		PoolSize:        redisPoolSize,
		MinIdleConns:    redisMinIdleConns,
		DialTimeout:     redisDialTimeout,
		ReadTimeout:     redisReadTimeout,
		WriteTimeout:    redisWriteTimeout,
		PoolTimeout:     4 * time.Second,
		MaxRetries:      3,
		MinRetryBackoff: 8 * time.Millisecond,
		MaxRetryBackoff: 512 * time.Millisecond,
	})
	defer rdb.Close()

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Println("Successfully connected to Redis")

	rl := limiter.NewRateLimiter(rdb, rateLimit, windowSize)

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ctx := r.Context()
		key := getClientIP(r)

		redisStart := time.Now()
		result, err := rl.Allow(ctx, key)
		redisLatency.Observe(time.Since(redisStart).Seconds())

		if err != nil {
			duration := time.Since(start).Seconds()
			requestLatency.WithLabelValues("500").Observe(duration)
			requestsTotal.WithLabelValues("500").Inc()
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		setRateLimitHeaders(w, result)

		if !result.Allowed {
			retryAfter := int(math.Ceil(time.Until(result.ResetAt).Seconds()))
			if retryAfter < 1 {
				retryAfter = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))

			duration := time.Since(start).Seconds()
			requestLatency.WithLabelValues("429").Observe(duration)
			requestsTotal.WithLabelValues("429").Inc()
			rateLimitHits.Inc()
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		duration := time.Since(start).Seconds()
		requestLatency.WithLabelValues("200").Observe(duration)
		requestsTotal.WithLabelValues("200").Inc()
		fmt.Fprintln(w, "Request allowed")
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		health := HealthResponse{
			Status: "healthy",
			Redis:  "connected",
		}

		if err := rdb.Ping(ctx).Err(); err != nil {
			health.Status = "unhealthy"
			health.Redis = "disconnected"
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(health)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(health)
	})

	mux.Handle("/metrics", promhttp.Handler())

	addr := fmt.Sprintf(":%s", port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}()

	log.Printf("Rate limiter service started on %s", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
	log.Println("Server stopped gracefully")
}
