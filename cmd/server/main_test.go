package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewHTTPServerLimits(t *testing.T) {
	srv := newHTTPServer(":8080", http.NewServeMux())

	if srv.Addr != ":8080" {
		t.Errorf("Addr: got %q, want %q", srv.Addr, ":8080")
	}
	if srv.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("ReadHeaderTimeout: got %v, want %v", srv.ReadHeaderTimeout, 5*time.Second)
	}
	if srv.MaxHeaderBytes != 64<<10 {
		t.Errorf("MaxHeaderBytes: got %d, want %d", srv.MaxHeaderBytes, 64<<10)
	}
	if srv.ReadTimeout != 30*time.Second {
		t.Errorf("ReadTimeout: got %v, want %v", srv.ReadTimeout, 30*time.Second)
	}
	if srv.WriteTimeout != 60*time.Second {
		t.Errorf("WriteTimeout: got %v, want %v", srv.WriteTimeout, 60*time.Second)
	}
	if srv.IdleTimeout != 120*time.Second {
		t.Errorf("IdleTimeout: got %v, want %v", srv.IdleTimeout, 120*time.Second)
	}
}

func TestReadyHandler_Healthy(t *testing.T) {
	imgproxySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer imgproxySrv.Close()

	client := imgproxySrv.Client()
	handler := newReadyHandler(imgproxySrv.URL, client)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestReadyHandler_Unhealthy(t *testing.T) {
	imgproxySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer imgproxySrv.Close()

	client := imgproxySrv.Client()
	handler := newReadyHandler(imgproxySrv.URL, client)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
