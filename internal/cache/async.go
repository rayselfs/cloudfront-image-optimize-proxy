package cache

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/rayselfs/cloudfront-image-optimize-proxy/internal/metrics"
)

type asyncPutCache struct {
	inner   Cache
	timeout time.Duration
}

// WrapAsyncPut wraps c so that Put is executed in a background goroutine.
// The response is never blocked waiting for S3. Put errors are logged but never returned.
func WrapAsyncPut(c Cache, timeout time.Duration) Cache {
	return &asyncPutCache{inner: c, timeout: timeout}
}

func (a *asyncPutCache) Get(ctx context.Context, key string) (io.ReadCloser, string, error) {
	return a.inner.Get(ctx, key)
}

// Put fires a goroutine to write body to the inner cache. It always returns nil immediately.
// The body reader must remain valid until the goroutine completes; callers should pass
// a bytes.NewReader (already-buffered) rather than a streaming body.
func (a *asyncPutCache) Put(_ context.Context, key string, body io.Reader, contentType string) error {
	go func() {
		putCtx, cancel := context.WithTimeout(context.Background(), a.timeout)
		defer cancel()
		if err := a.inner.Put(putCtx, key, body, contentType); err != nil {
			slog.Error("async cache put failed", "key", key, "error", err)
			metrics.IncPutError()
		}
	}()
	return nil
}
