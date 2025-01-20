package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

func main() {
	url := "http://localhost:8081"
	concurrency := 100
	requests := 1000

	var wg sync.WaitGroup
	results := make(chan bool, requests)

	start := time.Now()

	// Launch concurrent requests
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < requests/concurrency; j++ {
				resp, err := http.Get(url)
				if err != nil {
					fmt.Printf("Error: %v\n", err)
					results <- false
					continue
				}
				results <- resp.StatusCode == http.StatusOK
				resp.Body.Close()
			}
		}()
	}

	// Wait for all requests and collect results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Count results
	success := 0
	limited := 0
	for result := range results {
		if result {
			success++
		} else {
			limited++
		}
	}

	duration := time.Since(start)
	fmt.Printf("Load Test Results:\n")
	fmt.Printf("Total Requests: %d\n", requests)
	fmt.Printf("Successful: %d\n", success)
	fmt.Printf("Rate Limited: %d\n", limited)
	fmt.Printf("Duration: %v\n", duration)
	fmt.Printf("Requests/sec: %.2f\n", float64(requests)/duration.Seconds())
}
