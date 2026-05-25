package metrics

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

var (
	requestsTotal  atomic.Int64
	cacheHits      atomic.Int64
	cacheMisses    atomic.Int64
	cacheBypasses  atomic.Int64
	imgproxyErrors atomic.Int64
	putErrors      atomic.Int64
)

func IncRequest()       { requestsTotal.Add(1) }
func IncCacheHit()      { cacheHits.Add(1) }
func IncCacheMiss()     { cacheMisses.Add(1) }
func IncCacheBypass()   { cacheBypasses.Add(1) }
func IncImgproxyError() { imgproxyErrors.Add(1) }
func IncPutError()      { putErrors.Add(1) }

// Reset zeroes all counters. Intended for use in tests only.
func Reset() {
	requestsTotal.Store(0)
	cacheHits.Store(0)
	cacheMisses.Store(0)
	cacheBypasses.Store(0)
	imgproxyErrors.Store(0)
	putErrors.Store(0)
}

func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		writeCounter(w, "http_requests_total", "Total image proxy requests", requestsTotal.Load())
		writeCounter(w, "cache_hits_total", "Cache HIT responses served", cacheHits.Load())
		writeCounter(w, "cache_misses_total", "Cache MISS responses after transform", cacheMisses.Load())
		writeCounter(w, "cache_bypasses_total", "Requests bypassed due to non-image content", cacheBypasses.Load())
		writeCounter(w, "imgproxy_errors_total", "imgproxy transform failures (fell back to original)", imgproxyErrors.Load())
		writeCounter(w, "cache_put_errors_total", "S3 cache write failures", putErrors.Load())
	})
}

func writeCounter(w http.ResponseWriter, name, help string, value int64) {
	fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n%s %d\n", name, help, name, name, value)
}
