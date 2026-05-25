package upstream

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Resolver determines the upstream source for an image.
type Resolver interface {
	Resolve(r *http.Request) (sourceURL string, fetchFunc func() (io.ReadCloser, string, error), err error)
}

type s3Presigner interface {
	PresignGetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

var newS3Presigner = func(ctx context.Context) (s3Presigner, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	return s3.NewPresignClient(s3.NewFromConfig(cfg)), nil
}

// DefaultResolver resolves image sources from either S3 or the upstream gateway.
type DefaultResolver struct {
	httpClient      *http.Client
	allowedGateways []string
	// presigner is initialized once via presignerOnce to avoid calling
	// awsconfig.LoadDefaultConfig on every S3 request.
	presignerOnce sync.Once
	presigner     s3Presigner
	presignerErr  error
}

// NewResolver creates the default upstream resolver with the given HTTP timeout.
func NewResolver(timeout time.Duration, allowedGateways []string) *DefaultResolver {
	return &DefaultResolver{
		httpClient:      &http.Client{Timeout: timeout},
		allowedGateways: allowedGateways,
	}
}

// Resolve returns the source URL that imgproxy should read and a fallback fetch function.
func (d *DefaultResolver) Resolve(r *http.Request) (string, func() (io.ReadCloser, string, error), error) {
	if r.Header.Get("X-Img-Source-Type") == "s3" {
		return d.resolveS3(r)
	}

	rawGateway := strings.TrimSpace(r.Header.Get("X-Img-Upstream-Gateway"))
	if rawGateway == "" {
		return "", nil, fmt.Errorf("X-Img-Upstream-Gateway header is required")
	}

	if !strings.Contains(rawGateway, "://") {
		rawGateway = "http://" + rawGateway
	}
	gatewayURL, err := url.Parse(rawGateway)
	if err != nil || gatewayURL.Host == "" {
		return "", nil, fmt.Errorf("X-Img-Upstream-Gateway has invalid value")
	}

	if len(d.allowedGateways) > 0 {
		allowed := false
		for _, g := range d.allowedGateways {
			if gatewayURL.Host == g {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", nil, fmt.Errorf("upstream gateway not in allowlist")
		}
	}

	sourceURL := gatewayURL.Scheme + "://" + gatewayURL.Host + requestURI(r)
	fetchFunc := func() (io.ReadCloser, string, error) {
		return d.fetchHTTP(r.Context(), sourceURL, r.Host)
	}

	return sourceURL, fetchFunc, nil
}

func (d *DefaultResolver) resolveS3(r *http.Request) (string, func() (io.ReadCloser, string, error), error) {
	bucket := strings.TrimSpace(r.Header.Get("X-Img-Source-Bucket"))
	if bucket == "" {
		return "", nil, fmt.Errorf("X-Img-Source-Bucket is required for s3 source")
	}

	// Initialize the presigner once per resolver instance.
	d.presignerOnce.Do(func() {
		d.presigner, d.presignerErr = newS3Presigner(r.Context())
	})
	if d.presignerErr != nil {
		return "", nil, d.presignerErr
	}

	presigned, err := d.presigner.PresignGetObject(r.Context(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(strings.TrimPrefix(r.URL.Path, "/")),
	})
	if err != nil {
		return "", nil, err
	}

	sourceURL := presigned.URL
	fetchFunc := func() (io.ReadCloser, string, error) {
		return d.fetchHTTP(r.Context(), sourceURL, "")
	}

	return sourceURL, fetchFunc, nil
}

func requestURI(r *http.Request) string {
	uri := r.URL.RequestURI()
	if uri == "" {
		return "/"
	}
	return uri
}

func (d *DefaultResolver) fetchHTTP(ctx context.Context, url, host string) (io.ReadCloser, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	if host != "" {
		req.Host = host
	}

	res, err := d.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		res.Body.Close()
		return nil, "", fmt.Errorf("upstream returned status %d", res.StatusCode)
	}

	return res.Body, res.Header.Get("Content-Type"), nil
}
