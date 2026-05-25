package cache

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/rayselfs/cloudfront-image-optimize-proxy/internal/metrics"
)

// AsyncPutCache wraps a Cache and executes Put operations in background goroutines.
// Use Wait to drain in-flight puts during graceful shutdown.
type AsyncPutCache struct {
	inner   Cache
	timeout time.Duration
	sem     chan struct{}
	wg      sync.WaitGroup
}

// WrapAsyncPut wraps c so that Put is executed in a background goroutine.
// Concurrency is capped at maxConcurrency; excess puts are dropped and logged.
func WrapAsyncPut(c Cache, timeout time.Duration, maxConcurrency int) *AsyncPutCache {
	return &AsyncPutCache{
		inner:   c,
		timeout: timeout,
		sem:     make(chan struct{}, maxConcurrency),
	}
}

func (a *AsyncPutCache) Get(ctx context.Context, key string) (io.ReadCloser, string, error) {
	return a.inner.Get(ctx, key)
}

// Put fires a background goroutine to write body to the inner cache.
// Always returns nil immediately. body must be fully buffered (e.g. bytes.NewReader).
func (a *AsyncPutCache) Put(_ context.Context, key string, body io.Reader, contentType string) error {
	select {
	case a.sem <- struct{}{}:
	default:
		slog.Warn("async cache put dropped: worker pool full", "key", key)
		metrics.IncPutError()
		return nil
	}

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer func() { <-a.sem }()

		putCtx, cancel := context.WithTimeout(context.Background(), a.timeout)
		defer cancel()
		if err := a.inner.Put(putCtx, key, body, contentType); err != nil {
			slog.Error("async cache put failed", "key", key, "error", err)
			metrics.IncPutError()
		}
	}()
	return nil
}

// Wait blocks until all in-flight Put goroutines complete.
func (a *AsyncPutCache) Wait() {
	a.wg.Wait()
}
