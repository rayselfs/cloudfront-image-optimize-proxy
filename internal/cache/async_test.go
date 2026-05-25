package cache

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"
	"time"
)

type syncCache struct {
	mu      sync.Mutex
	putKey  string
	putData []byte
	putCT   string
	putErr  error
	calls   int
}

func (s *syncCache) Get(_ context.Context, _ string) (io.ReadCloser, string, error) {
	return nil, "", ErrNotFound
}

func (s *syncCache) Put(_ context.Context, key string, body io.Reader, contentType string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.putKey = key
	s.putCT = contentType
	data, _ := io.ReadAll(body)
	s.putData = data
	return s.putErr
}

func TestWrapAsyncPutReturnsImmediately(t *testing.T) {
	inner := &syncCache{}
	wrapped := WrapAsyncPut(inner, 5*time.Second)

	start := time.Now()
	err := wrapped.Put(context.Background(), "key", bytes.NewReader([]byte("data")), "image/webp")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Put returned error: %v", err)
	}
	// The wrapper should return well under 1 second (not waiting for inner.Put).
	if elapsed > 500*time.Millisecond {
		t.Errorf("Put took %v, expected near-instant return", elapsed)
	}
}

func TestWrapAsyncPutDelegatesData(t *testing.T) {
	inner := &syncCache{}
	wrapped := WrapAsyncPut(inner, 5*time.Second)

	data := []byte("hello-image")
	_ = wrapped.Put(context.Background(), "mykey", bytes.NewReader(data), "image/avif")

	// Wait for goroutine to complete.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		inner.mu.Lock()
		calls := inner.calls
		inner.mu.Unlock()
		if calls > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	inner.mu.Lock()
	defer inner.mu.Unlock()
	if inner.calls != 1 {
		t.Fatalf("inner.Put calls = %d, want 1", inner.calls)
	}
	if inner.putKey != "mykey" {
		t.Errorf("key = %q, want mykey", inner.putKey)
	}
	if !bytes.Equal(inner.putData, data) {
		t.Errorf("data = %q, want %q", inner.putData, data)
	}
	if inner.putCT != "image/avif" {
		t.Errorf("contentType = %q, want image/avif", inner.putCT)
	}
}

func TestWrapAsyncPutGetPassThrough(t *testing.T) {
	inner := &syncCache{}
	wrapped := WrapAsyncPut(inner, 5*time.Second)

	_, _, err := wrapped.Get(context.Background(), "key")
	if err != ErrNotFound {
		t.Errorf("Get err = %v, want ErrNotFound", err)
	}
}
