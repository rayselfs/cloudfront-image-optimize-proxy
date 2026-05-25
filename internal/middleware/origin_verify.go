package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"log/slog"
	"net/http"
)

func CloudFrontVerify(secrets []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/health", "/ready", "/metrics":
				next.ServeHTTP(w, r)
				return
			}

			if len(secrets) == 0 {
				slog.Warn("origin verify: disabled, all requests allowed")
				next.ServeHTTP(w, r)
				return
			}

			tokenHash := sha256.Sum256([]byte(r.Header.Get("X-Origin-Verify")))
			for _, s := range secrets {
				secretHash := sha256.Sum256([]byte(s))
				if subtle.ConstantTimeCompare(tokenHash[:], secretHash[:]) == 1 {
					next.ServeHTTP(w, r)
					return
				}
			}

			slog.Warn("origin verify: unauthorized access attempt",
				"remote_addr", r.RemoteAddr,
				"path", r.URL.Path,
				"method", r.Method,
			)
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		})
	}
}
