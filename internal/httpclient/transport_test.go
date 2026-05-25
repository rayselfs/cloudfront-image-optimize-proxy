package httpclient

import (
	"testing"
	"time"
)

func TestNewTransport(t *testing.T) {
	tr := NewTransport()
	if tr.MaxIdleConns != 100 {
		t.Fatalf("MaxIdleConns = %d, want 100", tr.MaxIdleConns)
	}
	if tr.MaxIdleConnsPerHost != 20 {
		t.Fatalf("MaxIdleConnsPerHost = %d, want 20", tr.MaxIdleConnsPerHost)
	}
	if tr.IdleConnTimeout != 90*time.Second {
		t.Fatalf("IdleConnTimeout = %v, want 90s", tr.IdleConnTimeout)
	}
	if tr.TLSHandshakeTimeout != 10*time.Second {
		t.Fatalf("TLSHandshakeTimeout = %v, want 10s", tr.TLSHandshakeTimeout)
	}
}
