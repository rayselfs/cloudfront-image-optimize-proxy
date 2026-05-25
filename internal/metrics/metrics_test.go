package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func resetCounters() {
	requestsTotal.Store(0)
	cacheHits.Store(0)
	cacheMisses.Store(0)
	cacheBypasses.Store(0)
	imgproxyErrors.Store(0)
	putErrors.Store(0)
}

func TestIncrements(t *testing.T) {
	resetCounters()

	IncRequest()
	IncRequest()
	IncCacheHit()
	IncCacheMiss()
	IncCacheMiss()
	IncCacheBypass()
	IncImgproxyError()
	IncPutError()

	if got := requestsTotal.Load(); got != 2 {
		t.Errorf("requestsTotal = %d, want 2", got)
	}
	if got := cacheHits.Load(); got != 1 {
		t.Errorf("cacheHits = %d, want 1", got)
	}
	if got := cacheMisses.Load(); got != 2 {
		t.Errorf("cacheMisses = %d, want 2", got)
	}
	if got := cacheBypasses.Load(); got != 1 {
		t.Errorf("cacheBypasses = %d, want 1", got)
	}
	if got := imgproxyErrors.Load(); got != 1 {
		t.Errorf("imgproxyErrors = %d, want 1", got)
	}
	if got := putErrors.Load(); got != 1 {
		t.Errorf("putErrors = %d, want 1", got)
	}
}

func TestHandlerPrometheusFormat(t *testing.T) {
	resetCounters()
	IncRequest()
	IncCacheHit()
	IncCacheMiss()

	w := httptest.NewRecorder()
	Handler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))

	body := w.Body.String()
	for _, want := range []string{
		"# HELP http_requests_total",
		"# TYPE http_requests_total counter",
		"http_requests_total 1",
		"cache_hits_total 1",
		"cache_misses_total 1",
		"cache_bypasses_total 0",
		"imgproxy_errors_total 0",
		"cache_put_errors_total 0",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics output missing %q\nfull body:\n%s", want, body)
		}
	}

	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain prefix", ct)
	}
}
