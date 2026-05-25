package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/rayselfs/cloudfront-image-optimize-proxy/internal/metrics"
)

// Logging is a structured JSON logging middleware.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		start := time.Now()

		next.ServeHTTP(wrapped, r)

		xCache := wrapped.Header().Get("X-Cache")
		switch xCache {
		case "HIT":
			metrics.IncCacheHit()
		case "MISS":
			metrics.IncCacheMiss()
		case "BYPASS":
			metrics.IncCacheBypass()
		}
		metrics.IncRequest()

		slog.Info("request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration_ms", time.Since(start).Milliseconds(),
			"x_cache", xCache,
			"bytes_written", wrapped.bytesWritten,
			"remote_addr", r.RemoteAddr,
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (w *responseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.bytesWritten += n
	return n, err
}
