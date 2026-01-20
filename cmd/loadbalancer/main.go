package main

import (
	"context"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "load_balancer_requests_total",
			Help: "Total number of requests handled by the load balancer.",
		},
		[]string{"backend", "status"},
	)
	requestLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "load_balancer_request_duration_seconds",
			Help:    "Request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"backend"},
	)
	backendHealth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "load_balancer_backend_healthy",
			Help: "Backend health status (1=healthy, 0=unhealthy).",
		},
		[]string{"backend"},
	)
)

func init() {
	prometheus.MustRegister(requestsTotal)
	prometheus.MustRegister(requestLatency)
	prometheus.MustRegister(backendHealth)
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

	val := 0.0
	if alive {
		val = 1.0
	}
	backendHealth.WithLabelValues(b.URL.Host).Set(val)
}

func (b *Backend) IsAlive() bool {
	b.mu.RLock()
	alive := b.Alive
	b.mu.RUnlock()
	return alive
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func healthCheck(backend *Backend) {
	client := &http.Client{Timeout: 5 * time.Second}

	for {
		start := time.Now()
		resp, err := client.Get(backend.URL.String() + "/health")
		duration := time.Since(start)

		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				backend.SetAlive(true)
				log.Printf("Backend %s is healthy (response time: %v)", backend.URL.Host, duration)
			} else {
				backend.SetAlive(false)
				log.Printf("Backend %s is unhealthy (status: %d)", backend.URL.Host, resp.StatusCode)
			}
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

	randomWeight := rand.IntN(totalWeight)
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
		backendHealth.WithLabelValues(backend.URL.Host).Set(1)
		go healthCheck(backend)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		backend := getNextBackend(backends)

		if backend == nil {
			requestsTotal.WithLabelValues("none", "503").Inc()
			http.Error(w, "No healthy backends", http.StatusServiceUnavailable)
			return
		}

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		backend.Proxy.ServeHTTP(wrapped, r)

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(wrapped.statusCode)
		requestsTotal.WithLabelValues(backend.URL.Host, status).Inc()
		requestLatency.WithLabelValues(backend.URL.Host).Observe(duration)
	})

	mux.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down load balancer...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("Shutdown error: %v", err)
		}
	}()

	log.Println("Load balancer started on :8080")
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
	log.Println("Load balancer stopped gracefully")
}
