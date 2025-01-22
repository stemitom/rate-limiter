package main

import (
	"log"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var requestsTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "load_balancer_requests_total",
		Help: "Total number of requests handled by the load balancer.",
	},
	[]string{"backend", "status"},
)

func init() {
	prometheus.MustRegister(requestsTotal)
}

type Backend struct {
	URL    *url.URL
	Proxy  *httputil.ReverseProxy
	Weight int
	mu     sync.RWMutex
	Alive  bool
}

func (b *Backend) SetAlive(alive bool) {
	b.mu.Lock()
	b.Alive = alive
	b.mu.Unlock()
}

func (b *Backend) IsAlive() bool {
	b.mu.RLock()
	alive := b.Alive
	b.mu.RUnlock()
	return alive
}

func healthCheck(backend *Backend) {
	for {
		start := time.Now()
		resp, err := http.Get(backend.URL.String() + "/health")
		duration := time.Since(start)

		if err == nil && resp.StatusCode == http.StatusOK {
			backend.SetAlive(true)
			log.Printf("Backend %s is healthy (response time: %v)", backend.URL.Host, duration)
		} else {
			backend.SetAlive(false)
			log.Printf("Backend %s is unhealthy (error: %v)", backend.URL.Host, err)
		}
		time.Sleep(10 * time.Second)
	}
}

func getNextBackend(backends []*Backend) *Backend {
	totalWeight := 0
	for _, backend := range backends {
		if backend.IsAlive() {
			totalWeight += backend.Weight
		}
	}

	if totalWeight == 0 {
		return nil
	}

	// Select a backend based on weight
	randomWeight := rand.Intn(totalWeight)
	for _, backend := range backends {
		if backend.IsAlive() {
			randomWeight -= backend.Weight
			if randomWeight < 0 {
				return backend
			}
		}
	}

	return nil
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func main() {
	rand.Seed(time.Now().UnixNano())

	backend1URL := getEnv("BACKEND_1_URL", "localhost:8081")
	backend2URL := getEnv("BACKEND_2_URL", "localhost:8082")
	backend1Weight, _ := strconv.Atoi(getEnv("BACKEND_1_WEIGHT", "2"))
	backend2Weight, _ := strconv.Atoi(getEnv("BACKEND_2_WEIGHT", "1"))

	backends := []*Backend{
		{URL: &url.URL{Scheme: "http", Host: backend1URL}, Alive: true, Weight: backend1Weight},
		{URL: &url.URL{Scheme: "http", Host: backend2URL}, Alive: true, Weight: backend2Weight},
	}

	for _, backend := range backends {
		backend.Proxy = httputil.NewSingleHostReverseProxy(backend.URL)
		go healthCheck(backend)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		backend := getNextBackend(backends)
		if backend != nil {
			requestsTotal.WithLabelValues(
				backend.URL.Host,
				"200",
			).Inc()
			backend.Proxy.ServeHTTP(w, r)
			return
		}
		requestsTotal.WithLabelValues("none", "503").Inc()
		http.Error(w, "No healthy backends", http.StatusServiceUnavailable)
	})

	http.Handle("/metrics", promhttp.Handler())

	log.Println("Load balancer started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
