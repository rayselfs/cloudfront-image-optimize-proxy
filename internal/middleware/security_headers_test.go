package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	tests := []struct {
		name          string
		header        string
		expectedValue string
	}{
		{
			name:          "X-Content-Type-Options",
			header:        "X-Content-Type-Options",
			expectedValue: "nosniff",
		},
		{
			name:          "X-Frame-Options",
			header:        "X-Frame-Options",
			expectedValue: "DENY",
		},
		{
			name:          "Referrer-Policy",
			header:        "Referrer-Policy",
			expectedValue: "no-referrer",
		},
		{
			name:          "Permissions-Policy",
			header:        "Permissions-Policy",
			expectedValue: "geolocation=(), microphone=(), camera=()",
		},
	}

	// Create a simple handler that responds with 200 OK
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	// Test GET /health
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.Header().Get(tt.header)
			if got != tt.expectedValue {
				t.Errorf("header %q = %q, want %q", tt.header, got, tt.expectedValue)
			}
		})
	}
}
