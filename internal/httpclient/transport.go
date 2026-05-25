package httpclient

import (
	"net/http"
	"time"
)

// NewTransport returns a tuned *http.Transport suitable for service-to-service
// HTTP connections (imgproxy sidecar and upstream gateway).
func NewTransport() *http.Transport {
	return &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}
}
