package middleware

import (
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

			token := []byte(r.Header.Get("X-Origin-Verify"))
			for _, s := range secrets {
				if subtle.ConstantTimeCompare(token, []byte(s)) == 1 {
					next.ServeHTTP(w, r)
					return
				}
			}

			slog.Warn("origin verify: unauthorized access attempt",
				"remote_addr", r.RemoteAddr,
				"x_forwarded_for", r.Header.Get("X-Forwarded-For"),
				"path", r.URL.Path,
				"method", r.Method,
			)
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		})
	}
}
