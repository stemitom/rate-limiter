package main

import (
	"flag"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type Result struct {
	err        error
	statusCode int
	duration   time.Duration
}

func main() {
	rps := flag.Int("rps", 100, "Desired requests per second")
	duration := flag.Duration("duration", 10*time.Second, "Test duration")
	flag.Parse()

	url := "http://localhost:8081"

	totalRequests := int(*rps * int(duration.Seconds()))

	results := make(chan Result, totalRequests)
	var wg sync.WaitGroup

	var currentSecond atomic.Int64
	var requestsThisSecond atomic.Int32

	fmt.Printf("Starting load test with %d RPS for %v...\n", *rps, *duration)

	start := time.Now()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	go func() {
		currentSecond.Store(time.Now().Unix())

		for time.Since(start) < *duration {
			now := time.Now().Unix()
			if now != currentSecond.Load() {
				currentSecond.Store(now)
				requestsThisSecond.Store(0)
			}

			if requestsThisSecond.Load() < int32(*rps) {
				wg.Add(1)
				go func() {
					defer wg.Done()

					reqStart := time.Now()
					resp, err := http.Get(url)
					reqDuration := time.Since(reqStart)
					if err != nil {
						results <- Result{statusCode: 0, duration: reqDuration, err: err}
						return
					}
					defer resp.Body.Close()

					results <- Result{statusCode: resp.StatusCode, duration: reqDuration, err: nil}
				}()

				requestsThisSecond.Add(1)
			}

			// Small sleep to prevent CPU spinning
			time.Sleep(time.Millisecond)
		}

		wg.Wait()
		close(results)
	}()

	var (
		success    int
		limited    int
		errors     int
		totalTime  time.Duration
		maxLatency time.Duration
		minLatency = time.Hour
	)

	for result := range results {
		if result.err != nil {
			errors++
			fmt.Printf("Error: %v\n", result.err)
			continue
		}

		totalTime += result.duration
		if result.duration > maxLatency {
			maxLatency = result.duration
		}
		if result.duration < minLatency {
			minLatency = result.duration
		}

		switch result.statusCode {
		case http.StatusOK:
			success++
		case http.StatusTooManyRequests:
			limited++
		default:
			errors++
		}
	}

	actualDuration := time.Since(start)
	actualRPS := float64(success+limited) / actualDuration.Seconds()

	fmt.Printf("\nLoad Test Results:\n")
	fmt.Printf("Duration: %v\n", actualDuration.Round(time.Millisecond))
	fmt.Printf("Total Requests: %d\n", success+limited+errors)
	fmt.Printf("Successful: %d\n", success)
	fmt.Printf("Rate Limited: %d\n", limited)
	fmt.Printf("Errors: %d\n", errors)
	fmt.Printf("Actual RPS: %.2f\n", actualRPS)
	fmt.Printf("Latency:\n")
	fmt.Printf("  Min: %v\n", minLatency.Round(time.Microsecond))
	fmt.Printf("  Max: %v\n", maxLatency.Round(time.Microsecond))
	fmt.Printf("  Avg: %v\n", (totalTime / time.Duration(success+limited)).Round(time.Microsecond))
}
