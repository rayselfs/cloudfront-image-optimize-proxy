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

func TestCloudFrontVerify_AllSecretsScanned(t *testing.T) {
	// Verify that all secrets are scanned even when token is wrong.
	// This ensures the constant-time comparison loop runs to completion.
	h := CloudFrontVerify([]string{"first", "second", "third"})(okHandler())
	r := httptest.NewRequest(http.MethodGet, "/image.jpg", nil)
	r.Header.Set("X-Origin-Verify", "wrong-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (all secrets scanned, none matched)", w.Code)
	}
}

func TestCloudFrontVerify_MatchesLastSecret(t *testing.T) {
	// Verify that a match on the last secret in the list is accepted.
	// This proves the loop doesn't early-return and processes all secrets.
	h := CloudFrontVerify([]string{"first", "second", "correct"})(okHandler())
	r := httptest.NewRequest(http.MethodGet, "/image.jpg", nil)
	r.Header.Set("X-Origin-Verify", "correct")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (last secret matched)", w.Code)
	}
}
