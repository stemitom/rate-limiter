package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/stemitom/rate-limiter/internal/limiter"
)

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func main() {
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	rateLimit, _ := strconv.Atoi(getEnv("RATE_LIMIT", "10"))
	windowSize, _ := time.ParseDuration(getEnv("WINDOW_SIZE", "1m"))
	port := getEnv("PORT", "8081")

	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	defer rdb.Close()

	limiter := limiter.NewRateLimiter(rdb, rateLimit, windowSize)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		key := r.RemoteAddr

		allowed, err := limiter.Allow(ctx, key)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if !allowed {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		fmt.Fprintln(w, "Request allowed")
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Rate limiter service started on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

