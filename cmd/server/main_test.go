package main

import (
	"net/http"
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
