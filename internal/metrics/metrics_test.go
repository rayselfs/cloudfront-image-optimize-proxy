package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCounterIncrement(t *testing.T) {
	Reset()

	IncRequest()
	IncCacheHit()
	IncCacheMiss()
	IncCacheBypass()
	IncImgproxyError()
	IncPutError()

	w := httptest.NewRecorder()
	Handler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	body := w.Body.String()

	for _, want := range []string{
		"http_requests_total 1",
		"cache_hits_total 1",
		"cache_misses_total 1",
		"cache_bypasses_total 1",
		"imgproxy_errors_total 1",
		"cache_put_errors_total 1",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in output:\n%s", want, body)
		}
	}
}

func TestReset(t *testing.T) {
	Reset()
	IncRequest()
	IncCacheHit()
	Reset()

	w := httptest.NewRecorder()
	Handler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	body := w.Body.String()

	for _, want := range []string{
		"http_requests_total 0",
		"cache_hits_total 0",
		"cache_misses_total 0",
		"cache_bypasses_total 0",
		"imgproxy_errors_total 0",
		"cache_put_errors_total 0",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("after Reset, missing %q in output:\n%s", want, body)
		}
	}
}

func TestGoMetricsPresent(t *testing.T) {
	Reset()

	w := httptest.NewRecorder()
	Handler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	body := w.Body.String()

	if !strings.Contains(body, "go_goroutines") {
		t.Errorf("go_goroutines not found in metrics output:\n%s", body)
	}
}
