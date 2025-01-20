package main

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	requestsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"status"},
	)

	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rate_limiter_requests_total",
			Help: "Total number of requests processed by rate limiter",
		},
		[]string{"status"},
	)

	requestLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rate_limiter_request_duration_seconds",
			Help:    "Request latency distribution",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"status"},
	)
)

func init() {
	prometheus.MustRegister(requestsCounter)
	prometheus.MustRegister(requestsTotal)
	prometheus.MustRegister(requestLatency)
}

func handler(w http.ResponseWriter, r *http.Request) {
	requestsCounter.WithLabelValues("200").Inc()
	w.Write([]byte("Hello, world!"))
}

func main() {
	http.HandleFunc("/", handler)
	http.Handle("/metrics", promhttp.Handler())
	log.Println("Metrics service started on :8083")
	http.ListenAndServe(":8083", nil)
}
