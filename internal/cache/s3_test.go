package cache

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type mockS3 struct {
	getOutput *s3.GetObjectOutput
	getErr    error
	putInput  *s3.PutObjectInput
	putErr    error
}

func (m *mockS3) GetObject(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	return m.getOutput, m.getErr
}

func (m *mockS3) PutObject(_ context.Context, params *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	m.putInput = params
	return &s3.PutObjectOutput{}, m.putErr
}

func TestKeyFromRequest(t *testing.T) {
	tests := []struct {
		host   string
		path   string
		params ImageParams
		want   string
	}{
		{"stream.viverse.com", "/assets/hero", ImageParams{640, "webp", 75}, "stream.viverse.com/assets/hero/640_webp_75"},
		{"stream.viverse.com", "assets/hero", ImageParams{640, "webp", 75}, "stream.viverse.com/assets/hero/640_webp_75"},
		{"example.com", "/img/logo.png", ImageParams{320, "jpeg", 90}, "example.com/img/logo.png/320_jpeg_90"},
	}
	for _, tt := range tests {
		got := KeyFromRequest(tt.host, tt.path, tt.params)
		if got != tt.want {
			t.Errorf("KeyFromRequest(%q, %q, %+v) = %q, want %q", tt.host, tt.path, tt.params, got, tt.want)
		}
	}
}

func TestCacheHit(t *testing.T) {
	body := io.NopCloser(strings.NewReader("image-data"))
	mock := &mockS3{
		getOutput: &s3.GetObjectOutput{
			Body:        body,
			ContentType: aws.String("image/webp"),
		},
	}
	c := NewS3Cache(mock, "test-bucket")
	rc, ct, err := c.Get(context.Background(), "some/key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ct != "image/webp" {
		t.Errorf("contentType = %q, want %q", ct, "image/webp")
	}
	rc.Close()
}

func TestCacheMiss(t *testing.T) {
	mock := &mockS3{getErr: &types.NoSuchKey{}}
	c := NewS3Cache(mock, "test-bucket")
	_, _, err := c.Get(context.Background(), "missing/key")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestCachePut(t *testing.T) {
	mock := &mockS3{}
	c := NewS3Cache(mock, "test-bucket")
	body := strings.NewReader("image-data")
	err := c.Put(context.Background(), "some/key", body, "image/webp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.putInput == nil {
		t.Fatal("PutObject was not called")
	}
	if aws.ToString(mock.putInput.Key) != "some/key" {
		t.Errorf("key = %q, want %q", aws.ToString(mock.putInput.Key), "some/key")
	}
	if aws.ToString(mock.putInput.ContentType) != "image/webp" {
		t.Errorf("contentType = %q, want %q", aws.ToString(mock.putInput.ContentType), "image/webp")
	}
	if aws.ToString(mock.putInput.CacheControl) != "public, max-age=31536000" {
		t.Errorf("cacheControl = %q, want %q", aws.ToString(mock.putInput.CacheControl), "public, max-age=31536000")
	}
}

func TestS3CachePutInvalidContentType(t *testing.T) {
	mock := &mockS3{}
	c := NewS3Cache(mock, "test-bucket")
	err := c.Put(context.Background(), "some/key", strings.NewReader("data"), "image/png\r\nX-Bad: 1")
	if err == nil {
		t.Fatal("expected error for CRLF content type, got nil")
	}
	if mock.putInput != nil {
		t.Fatal("PutObject should not be called when content type is invalid")
	}
}
