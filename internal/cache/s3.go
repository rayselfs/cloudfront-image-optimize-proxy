package cache

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Cache is the interface for image cache operations.
type Cache interface {
	Get(ctx context.Context, key string) (io.ReadCloser, string, error)
	Put(ctx context.Context, key string, body io.Reader, contentType string) error
}

// ErrNotFound is returned when a cache key does not exist.
var ErrNotFound = errors.New("cache: not found")

// ImageParams holds the image transformation parameters used to build a cache key.
type ImageParams struct {
	Width   int
	Format  string
	Quality int
}

// KeyFromRequest builds a cache key from host, path, and image params.
// Format: {host}/{path}/{width}_{format}_{quality}
func KeyFromRequest(host, path string, params ImageParams) string {
	path = strings.TrimPrefix(path, "/")
	return fmt.Sprintf("%s/%s/%d_%s_%d", host, path, params.Width, params.Format, params.Quality)
}

// S3API is the subset of the AWS S3 client used by S3Cache (enables mocking).
type S3API interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// S3Cache implements Cache backed by an S3 bucket.
type S3Cache struct {
	client S3API
	bucket string
}

// NewS3Cache creates a new S3Cache.
func NewS3Cache(client S3API, bucket string) *S3Cache {
	return &S3Cache{client: client, bucket: bucket}
}

// Get retrieves a cached object. Returns ErrNotFound if the key does not exist.
func (c *S3Cache) Get(ctx context.Context, key string) (io.ReadCloser, string, error) {
	out, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var noSuchKey *types.NoSuchKey
		if errors.As(err, &noSuchKey) {
			return nil, "", ErrNotFound
		}
		return nil, "", err
	}

	contentType := ""
	if out.ContentType != nil {
		contentType = *out.ContentType
	}
	return out.Body, contentType, nil
}

// Put stores an object in the cache with a long-lived Cache-Control header.
func (c *S3Cache) Put(ctx context.Context, key string, body io.Reader, contentType string) error {
	if strings.ContainsAny(contentType, "\r\n") {
		return fmt.Errorf("cache: content type contains illegal control characters")
	}
	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:       aws.String(c.bucket),
		Key:          aws.String(key),
		Body:         body,
		ContentType:  aws.String(contentType),
		CacheControl: aws.String("public, max-age=31536000"),
	})
	return err
}
