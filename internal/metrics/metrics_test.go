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

func TestHistogramObservation(t *testing.T) {
	Reset()

	ObserveHTTPRequest("GET", "2xx", "HIT", 0.05)
	ObserveImgproxy("2xx", 0.1)
	ObserveS3Get("hit", 0.002)
	ObserveS3Put("success", 0.003)
	ObserveUpstreamFetch("success", 0.08)

	w := httptest.NewRecorder()
	Handler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	body := w.Body.String()

	for _, want := range []string{
		"http_request_duration_seconds",
		"imgproxy_duration_seconds",
		"s3_get_duration_seconds",
		"s3_put_duration_seconds",
		"upstream_fetch_duration_seconds",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing histogram %q in output", want)
		}
	}
}

func TestHistogramLabels(t *testing.T) {
	Reset()

	ObserveHTTPRequest("GET", "2xx", "HIT", 0.01)
	ObserveHTTPRequest("POST", "4xx", "MISS", 0.02)
	ObserveHTTPRequest("GET", "5xx", "UNKNOWN", 0.03)
	ObserveImgproxy("2xx", 0.01)
	ObserveImgproxy("5xx", 0.02)
	ObserveS3Get("hit", 0.001)
	ObserveS3Get("miss", 0.001)
	ObserveS3Get("error", 0.001)
	ObserveS3Put("success", 0.001)
	ObserveS3Put("error", 0.001)
	ObserveUpstreamFetch("success", 0.01)
	ObserveUpstreamFetch("error", 0.01)

	w := httptest.NewRecorder()
	Handler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	body := w.Body.String()

	boundedLabels := []string{
		`method="GET"`,
		`method="POST"`,
		`status_class="2xx"`,
		`status_class="4xx"`,
		`status_class="5xx"`,
		`cache_status="HIT"`,
		`cache_status="MISS"`,
		`cache_status="UNKNOWN"`,
		`outcome="hit"`,
		`outcome="miss"`,
		`outcome="error"`,
		`outcome="success"`,
	}
	for _, label := range boundedLabels {
		if !strings.Contains(body, label) {
			t.Errorf("missing label %q in output", label)
		}
	}
}
