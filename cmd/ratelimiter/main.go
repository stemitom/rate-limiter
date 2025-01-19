package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/stemitom/rate-limiter/internal/limiter"
)

func main() {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer rdb.Close()

	limiter := limiter.NewRateLimiter(rdb, 10, time.Minute)

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

	log.Println("Rate limiter service started on :8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}
