package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Config holds the load test configuration parameters
type Config struct {
	Client   *http.Client
	URL      string
	RPS      int
	Duration time.Duration
}

// Result represents the outcome of a single request
type Result struct {
	Error      error
	StatusCode int
	Duration   time.Duration
}

// Stats tracks the aggregate test results
type Stats struct {
	Success    int
	Limited    int
	Errors     int
	TotalTime  time.Duration
	MaxLatency time.Duration
	MinLatency time.Duration
}

// LoadTester manages the load testing process
type LoadTester struct {
	stats  *Stats
	config Config
	wg     sync.WaitGroup
}

// NewLoadTester creates a new LoadTester instance
func NewLoadTester(config Config) *LoadTester {
	return &LoadTester{
		config: config,
		stats: &Stats{
			MinLatency: time.Hour,
		},
	}
}

// makeRequest performs a single HTTP request and records the result
func (lt *LoadTester) makeRequest(results chan<- Result) {
	defer lt.wg.Done()

	start := time.Now()
	resp, err := lt.config.Client.Get(lt.config.URL)
	duration := time.Since(start)

	if err != nil {
		results <- Result{StatusCode: 0, Duration: duration, Error: err}
		return
	}
	defer resp.Body.Close()

	results <- Result{StatusCode: resp.StatusCode, Duration: duration}
}

// Run executes the load test
func (lt *LoadTester) Run(ctx context.Context) error {
	results := make(chan Result, lt.config.RPS*int(lt.config.Duration.Seconds()))
	defer close(results)

	var currentSecond atomic.Int64
	var requestsThisSecond atomic.Int32
	currentSecond.Store(time.Now().Unix())

	start := time.Now()
	go lt.processResults(results)

	// Main test loop
	for time.Since(start) < lt.config.Duration {
		now := time.Now().Unix()
		if now != currentSecond.Load() {
			currentSecond.Store(now)
			requestsThisSecond.Store(0)
		}

		if requestsThisSecond.Load() < int32(lt.config.RPS) {
			lt.wg.Add(1)
			go lt.makeRequest(results)
			requestsThisSecond.Add(1)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			time.Sleep(time.Millisecond) // Prevent CPU spinning
		}
	}

	lt.wg.Wait()
	return nil
}

// processResults handles the test results
func (lt *LoadTester) processResults(results <-chan Result) {
	for result := range results {
		if result.Error != nil {
			lt.stats.Errors++
			fmt.Printf("Error: %v\n", result.Error)
			continue
		}

		lt.stats.TotalTime += result.Duration
		lt.updateLatencyStats(result.Duration)

		switch result.StatusCode {
		case http.StatusOK:
			lt.stats.Success++
		case http.StatusTooManyRequests:
			lt.stats.Limited++
		default:
			lt.stats.Errors++
		}
	}
}

// updateLatencyStats updates min/max latency statistics
func (lt *LoadTester) updateLatencyStats(duration time.Duration) {
	if duration > lt.stats.MaxLatency {
		lt.stats.MaxLatency = duration
	}
	if duration < lt.stats.MinLatency {
		lt.stats.MinLatency = duration
	}
}

// PrintStats displays the test results
func (lt *LoadTester) PrintStats(actualDuration time.Duration) {
	total := lt.stats.Success + lt.stats.Limited + lt.stats.Errors
	actualRPS := float64(lt.stats.Success+lt.stats.Limited) / actualDuration.Seconds()

	fmt.Printf("\nLoad Test Results:\n")
	fmt.Printf("Duration: %v\n", actualDuration.Round(time.Millisecond))
	fmt.Printf("Total Requests: %d\n", total)
	fmt.Printf("Successful: %d\n", lt.stats.Success)
	fmt.Printf("Rate Limited: %d\n", lt.stats.Limited)
	fmt.Printf("Errors: %d\n", lt.stats.Errors)
	fmt.Printf("Actual RPS: %.2f\n", actualRPS)
	fmt.Printf("Latency:\n")
	fmt.Printf("  Min: %v\n", lt.stats.MinLatency.Round(time.Microsecond))
	fmt.Printf("  Max: %v\n", lt.stats.MaxLatency.Round(time.Microsecond))
	if total > 0 {
		fmt.Printf("  Avg: %v\n", (lt.stats.TotalTime / time.Duration(total)).Round(time.Microsecond))
	}
}

func main() {
	rps := flag.Int("rps", 100, "Desired requests per second")
	duration := flag.Duration("duration", 10*time.Second, "Test duration")
	url := flag.String("url", "http://localhost:8080", "Target URL")
	flag.Parse()

	config := Config{
		URL:      *url,
		RPS:      *rps,
		Duration: *duration,
		Client:   &http.Client{},
	}

	tester := NewLoadTester(config)
	fmt.Printf("Starting load test with %d RPS for %v...\n", config.RPS, config.Duration)

	start := time.Now()
	ctx := context.Background()
	if err := tester.Run(ctx); err != nil {
		fmt.Printf("Test failed: %v\n", err)
		return
	}

	tester.PrintStats(time.Since(start))
}
