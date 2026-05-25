package upstream

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type mockPresigner struct {
	t        *testing.T
	bucket   string
	key      string
	response string
}

func (m mockPresigner) PresignGetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	m.t.Helper()
	if got := aws.ToString(params.Bucket); got != m.bucket {
		m.t.Fatalf("Bucket = %q, want %q", got, m.bucket)
	}
	if got := aws.ToString(params.Key); got != m.key {
		m.t.Fatalf("Key = %q, want %q", got, m.key)
	}
	return &v4.PresignedHTTPRequest{URL: m.response}, nil
}

func TestResolveS3(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("s3 image"))
	}))
	defer server.Close()

	originalNewS3Presigner := newS3Presigner
	newS3Presigner = func(ctx context.Context) (s3Presigner, error) {
		return mockPresigner{
			t:        t,
			bucket:   "source-bucket",
			key:      "images/cat.png",
			response: server.URL + "/presigned",
		}, nil
	}
	t.Cleanup(func() { newS3Presigner = originalNewS3Presigner })

	req := httptest.NewRequest(http.MethodGet, "http://origin.example/images/cat.png?width=640", nil)
	req.Header.Set("X-Img-Source-Type", "s3")
	req.Header.Set("X-Img-Source-Bucket", "source-bucket")

	sourceURL, fetchFunc, err := NewResolver(30 * time.Second, nil).Resolve(req)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if sourceURL != server.URL+"/presigned" {
		t.Fatalf("sourceURL = %q, want presigned URL", sourceURL)
	}

	body, contentType, err := fetchFunc()
	if err != nil {
		t.Fatalf("fetchFunc() error = %v", err)
	}
	defer body.Close()

	if contentType != "image/png" {
		t.Fatalf("contentType = %q, want %q", contentType, "image/png")
	}
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(data) != "s3 image" {
		t.Fatalf("body = %q, want %q", string(data), "s3 image")
	}
}

func TestResolveGatewayFromHeader(t *testing.T) {
	var gotHost string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
		if r.URL.RequestURI() != "/images/cat.jpg?width=640" {
			t.Fatalf("RequestURI = %q, want %q", r.URL.RequestURI(), "/images/cat.jpg?width=640")
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("gateway image"))
	}))
	defer server.Close()

	req := httptest.NewRequest(http.MethodGet, "http://assets.example/images/cat.jpg?width=640", nil)
	req.Host = "assets.example"
	req.Header.Set("X-Img-Upstream-Gateway", server.URL)

	sourceURL, fetchFunc, err := NewResolver(30 * time.Second, nil).Resolve(req)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if sourceURL != server.URL+"/images/cat.jpg?width=640" {
		t.Fatalf("sourceURL = %q, want gateway URL", sourceURL)
	}

	body, contentType, err := fetchFunc()
	if err != nil {
		t.Fatalf("fetchFunc() error = %v", err)
	}
	defer body.Close()

	if gotHost != "assets.example" {
		t.Fatalf("Host = %q, want %q", gotHost, "assets.example")
	}
	if contentType != "image/jpeg" {
		t.Fatalf("contentType = %q, want %q", contentType, "image/jpeg")
	}
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(data) != "gateway image" {
		t.Fatalf("body = %q, want %q", string(data), "gateway image")
	}
}

func TestResolveGatewayMissingHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://assets.example/images/cat.jpg", nil)

	_, _, err := NewResolver(30 * time.Second, nil).Resolve(req)
	if err == nil {
		t.Fatal("Resolve() error = nil, want error for missing X-Img-Upstream-Gateway")
	}
	if !strings.Contains(err.Error(), "X-Img-Upstream-Gateway") {
		t.Fatalf("error = %q, want message mentioning X-Img-Upstream-Gateway", err.Error())
	}
}

func TestResolveGatewayAllowlistAllowed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("allowed image"))
	}))
	defer server.Close()

	serverHost := strings.TrimPrefix(server.URL, "http://")

	req := httptest.NewRequest(http.MethodGet, "http://assets.example/img/cat.jpg", nil)
	req.Header.Set("X-Img-Upstream-Gateway", server.URL)

	_, _, err := NewResolver(30*time.Second, []string{serverHost}).Resolve(req)
	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil (allowed gateway)", err)
	}
}

func TestResolveGatewayAllowlistBlocked(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://assets.example/img/cat.jpg", nil)
	req.Header.Set("X-Img-Upstream-Gateway", "http://evil.example.com")

	_, _, err := NewResolver(30*time.Second, []string{"trusted.example.com"}).Resolve(req)
	if err == nil {
		t.Fatal("Resolve() error = nil, want error for blocked gateway")
	}
	if !strings.Contains(err.Error(), "allowlist") {
		t.Fatalf("error = %q, want message mentioning allowlist", err.Error())
	}
}

func TestResolveGatewayAllowlistEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("image"))
	}))
	defer server.Close()

	req := httptest.NewRequest(http.MethodGet, "http://assets.example/img/cat.jpg", nil)
	req.Header.Set("X-Img-Upstream-Gateway", server.URL)

	_, _, err := NewResolver(30*time.Second, nil).Resolve(req)
	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil (no allowlist)", err)
	}
}
