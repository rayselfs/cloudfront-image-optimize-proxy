package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestCloudFrontVerify_NoSecrets(t *testing.T) {
	h := CloudFrontVerify(nil)(okHandler())
	r := httptest.NewRequest(http.MethodGet, "/image.jpg", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestCloudFrontVerify_ValidToken(t *testing.T) {
	h := CloudFrontVerify([]string{"secret-abc"})(okHandler())
	r := httptest.NewRequest(http.MethodGet, "/image.jpg", nil)
	r.Header.Set("X-Origin-Verify", "secret-abc")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestCloudFrontVerify_WrongToken(t *testing.T) {
	h := CloudFrontVerify([]string{"secret-abc"})(okHandler())
	r := httptest.NewRequest(http.MethodGet, "/image.jpg", nil)
	r.Header.Set("X-Origin-Verify", "wrong-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestCloudFrontVerify_MissingToken(t *testing.T) {
	h := CloudFrontVerify([]string{"secret-abc"})(okHandler())
	r := httptest.NewRequest(http.MethodGet, "/image.jpg", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestCloudFrontVerify_DualSecret(t *testing.T) {
	h := CloudFrontVerify([]string{"new-secret", "old-secret"})(okHandler())

	for _, token := range []string{"new-secret", "old-secret"} {
		r := httptest.NewRequest(http.MethodGet, "/image.jpg", nil)
		r.Header.Set("X-Origin-Verify", token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("token=%q: status = %d, want 200", token, w.Code)
		}
	}
}

func TestCloudFrontVerify_HealthExempt(t *testing.T) {
	h := CloudFrontVerify([]string{"secret-abc"})(okHandler())
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (health exempt)", w.Code)
	}
}

func TestCloudFrontVerify_ReadyExempt(t *testing.T) {
	h := CloudFrontVerify([]string{"secret-abc"})(okHandler())
	r := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (ready exempt)", w.Code)
	}
}

func TestCloudFrontVerify_MetricsExempt(t *testing.T) {
	h := CloudFrontVerify([]string{"secret-abc"})(okHandler())
	r := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (metrics exempt)", w.Code)
	}
}
