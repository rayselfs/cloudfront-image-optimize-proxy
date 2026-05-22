package imgproxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// TransformParams holds parameters for image transformation.
type TransformParams struct {
	Width   int
	Format  string
	Quality int
}

// Transformer is the interface for image transformation.
type Transformer interface {
	Transform(ctx context.Context, sourceURL string, params TransformParams) (io.ReadCloser, string, error)
}

// Client is an HTTP client for imgproxy.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new imgproxy Client with the given base URL and HTTP timeout.
func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// buildProcessingURL constructs the imgproxy processing URL.
func buildProcessingURL(baseURL, sourceURL string, params TransformParams) string {
	format := params.Format
	if format == "jpeg" {
		format = "jpg"
	}
	return fmt.Sprintf("%s/unsafe/rs:fit:%d/q:%d/%s/plain/%s", baseURL, params.Width, params.Quality, format, sourceURL)
}

// Transform fetches the transformed image from imgproxy.
func (c *Client) Transform(ctx context.Context, sourceURL string, params TransformParams) (io.ReadCloser, string, error) {
	url := buildProcessingURL(c.baseURL, sourceURL, params)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("imgproxy: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("imgproxy: request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		snippet := make([]byte, 256)
		n, _ := resp.Body.Read(snippet)
		resp.Body.Close()
		return nil, "", fmt.Errorf("imgproxy: unexpected status %d: %s", resp.StatusCode, snippet[:n])
	}

	return resp.Body, resp.Header.Get("Content-Type"), nil
}
