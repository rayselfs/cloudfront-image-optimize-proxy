package imgproxy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBuildURL(t *testing.T) {
	tests := []struct {
		name     string
		params   TransformParams
		wantSufx string
	}{
		{"webp", TransformParams{Width: 800, Format: "webp", Quality: 85}, "/unsafe/rs:fit:800/q:85/webp/plain/http:%2F%2Fexample.com%2Fimg.png"},
		{"avif", TransformParams{Width: 400, Format: "avif", Quality: 70}, "/unsafe/rs:fit:400/q:70/avif/plain/http:%2F%2Fexample.com%2Fimg.png"},
		{"jpeg to jpg", TransformParams{Width: 1200, Format: "jpeg", Quality: 90}, "/unsafe/rs:fit:1200/q:90/jpg/plain/http:%2F%2Fexample.com%2Fimg.png"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildProcessingURL("http://localhost:8081", "http://example.com/img.png", tc.params)
			want := "http://localhost:8081" + tc.wantSufx
			if got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

func TestBuildURLEscaping(t *testing.T) {
	tests := []struct {
		name        string
		sourceURL   string
		wantEncoded string
	}{
		{
			name:        "space in source URL",
			sourceURL:   "https://example.com/image file.jpg",
			wantEncoded: "https:%2F%2Fexample.com%2Fimage%20file.jpg",
		},
		{
			name:        "percent in source URL",
			sourceURL:   "https://example.com/img%20x.jpg",
			wantEncoded: "https:%2F%2Fexample.com%2Fimg%2520x.jpg",
		},
		{
			name:        "query string in source URL",
			sourceURL:   "https://s3.amazonaws.com/bucket/img.jpg?X-Amz-Signature=a/b",
			wantEncoded: "https:%2F%2Fs3.amazonaws.com%2Fbucket%2Fimg.jpg%3FX-Amz-Signature=a%2Fb",
		},
		{
			name:        "slash in source URL path",
			sourceURL:   "https://example.com/path/to/image.jpg",
			wantEncoded: "https:%2F%2Fexample.com%2Fpath%2Fto%2Fimage.jpg",
		},
		{
			name:        "Unicode in source URL",
			sourceURL:   "https://example.com/图片.jpg",
			wantEncoded: "https:%2F%2Fexample.com%2F%E5%9B%BE%E7%89%87.jpg",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildProcessingURL("http://localhost:8081", tc.sourceURL, TransformParams{Width: 800, Format: "webp", Quality: 85})
			
			if !strings.HasSuffix(got, "/plain/"+tc.wantEncoded) {
				t.Errorf("got %q, want suffix /plain/%s", got, tc.wantEncoded)
			}
			
			plainIdx := strings.Index(got, "/plain/")
			if plainIdx != -1 && strings.Contains(got[plainIdx+7:], "?") {
				t.Errorf("raw '?' found after /plain/ in: %s", got)
			}
		})
	}
}

func TestTransformSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/webp")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fake-image-data"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, 30*time.Second)
	body, ct, err := c.Transform(context.Background(), "http://origin/img.png", TransformParams{Width: 800, Format: "webp", Quality: 85})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer body.Close()

	if ct != "image/webp" {
		t.Errorf("content-type: got %q, want %q", ct, "image/webp")
	}
	data, _ := io.ReadAll(body)
	if string(data) != "fake-image-data" {
		t.Errorf("body: got %q", data)
	}
}

func TestTransformError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, 30*time.Second)
	body, _, err := c.Transform(context.Background(), "http://origin/img.png", TransformParams{Width: 800, Format: "webp", Quality: 85})
	if err == nil {
		body.Close()
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention status 500, got: %v", err)
	}
}

func TestTransformTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	c := NewClient(srv.URL, 30*time.Second)
	_, _, err := c.Transform(ctx, "http://origin/img.png", TransformParams{Width: 800, Format: "webp", Quality: 85})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}
