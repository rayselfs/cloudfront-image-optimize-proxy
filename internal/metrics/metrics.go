package metrics

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	mu  sync.Mutex
	reg *prometheus.Registry

	requestsTotal  prometheus.Counter
	cacheHits      prometheus.Counter
	cacheMisses    prometheus.Counter
	cacheBypasses  prometheus.Counter
	imgproxyErrors prometheus.Counter
	putErrors      prometheus.Counter
)

func init() {
	initRegistry()
}

func initRegistry() {
	reg = prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	requestsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total image proxy requests",
	})
	cacheHits = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cache_hits_total",
		Help: "Cache HIT responses served",
	})
	cacheMisses = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cache_misses_total",
		Help: "Cache MISS responses after transform",
	})
	cacheBypasses = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cache_bypasses_total",
		Help: "Requests bypassed due to non-image content",
	})
	imgproxyErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "imgproxy_errors_total",
		Help: "imgproxy transform failures (fell back to original)",
	})
	putErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cache_put_errors_total",
		Help: "S3 cache write failures",
	})

	reg.MustRegister(
		requestsTotal,
		cacheHits,
		cacheMisses,
		cacheBypasses,
		imgproxyErrors,
		putErrors,
	)
}

// Reset re-creates the registry and all counters. Intended for use in tests only.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	initRegistry()
}

// Registry returns the package-level prometheus registry.
func Registry() *prometheus.Registry {
	mu.Lock()
	defer mu.Unlock()
	return reg
}

func IncRequest()       { mu.Lock(); requestsTotal.Inc(); mu.Unlock() }
func IncCacheHit()      { mu.Lock(); cacheHits.Inc(); mu.Unlock() }
func IncCacheMiss()     { mu.Lock(); cacheMisses.Inc(); mu.Unlock() }
func IncCacheBypass()   { mu.Lock(); cacheBypasses.Inc(); mu.Unlock() }
func IncImgproxyError() { mu.Lock(); imgproxyErrors.Inc(); mu.Unlock() }
func IncPutError()      { mu.Lock(); putErrors.Inc(); mu.Unlock() }

// Handler returns an HTTP handler that serves Prometheus metrics.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		h := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
		mu.Unlock()
		h.ServeHTTP(w, r)
	})
}
