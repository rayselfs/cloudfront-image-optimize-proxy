package config

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	t.Setenv("LISTEN_ADDR", "")
	t.Setenv("IMGPROXY_URL", "")
	t.Setenv("CACHE_S3_BUCKET", "source-images")
	t.Setenv("CACHE_S3_REGION", "")
	t.Setenv("MAX_WIDTH", "")
	t.Setenv("ALLOW_ALL_UPSTREAM_GATEWAYS", "true")
	t.Setenv("ALLOW_ALL_SOURCE_BUCKETS", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.ListenAddr != ":9999" {
		t.Fatalf("ListenAddr = %q, want %q", cfg.ListenAddr, ":9999")
	}
	if cfg.ImgproxyURL != "http://localhost:8081" {
		t.Fatalf("ImgproxyURL = %q, want %q", cfg.ImgproxyURL, "http://localhost:8081")
	}
	if cfg.CacheS3Bucket != "source-images" {
		t.Fatalf("CacheS3Bucket = %q, want %q", cfg.CacheS3Bucket, "source-images")
	}
	if cfg.CacheS3Region != "us-west-2" {
		t.Fatalf("CacheS3Region = %q, want %q", cfg.CacheS3Region, "us-west-2")
	}
	if cfg.MaxWidth != 1920 {
		t.Fatalf("MaxWidth = %d, want %d", cfg.MaxWidth, 1920)
	}
}

func TestRequiredCacheS3Bucket(t *testing.T) {
	t.Setenv("CACHE_S3_BUCKET", "")
	t.Setenv("ALLOW_ALL_UPSTREAM_GATEWAYS", "true")
	t.Setenv("ALLOW_ALL_SOURCE_BUCKETS", "true")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

func TestCustomConfig(t *testing.T) {
	t.Setenv("LISTEN_ADDR", ":9090")
	t.Setenv("IMGPROXY_URL", "http://imgproxy:8080")
	t.Setenv("CACHE_S3_BUCKET", "custom-bucket")
	t.Setenv("CACHE_S3_REGION", "ap-northeast-1")
	t.Setenv("MAX_WIDTH", "2048")
	t.Setenv("ALLOW_ALL_UPSTREAM_GATEWAYS", "true")
	t.Setenv("ALLOW_ALL_SOURCE_BUCKETS", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.ListenAddr != ":9090" {
		t.Fatalf("ListenAddr = %q, want %q", cfg.ListenAddr, ":9090")
	}
	if cfg.ImgproxyURL != "http://imgproxy:8080" {
		t.Fatalf("ImgproxyURL = %q, want %q", cfg.ImgproxyURL, "http://imgproxy:8080")
	}
	if cfg.CacheS3Bucket != "custom-bucket" {
		t.Fatalf("CacheS3Bucket = %q, want %q", cfg.CacheS3Bucket, "custom-bucket")
	}
	if cfg.CacheS3Region != "ap-northeast-1" {
		t.Fatalf("CacheS3Region = %q, want %q", cfg.CacheS3Region, "ap-northeast-1")
	}
	if cfg.MaxWidth != 2048 {
		t.Fatalf("MaxWidth = %d, want %d", cfg.MaxWidth, 2048)
	}
}

func TestLoadCSV_Empty(t *testing.T) {
	t.Setenv("CF_ORIGIN_SECRET", "")
	t.Setenv("CACHE_S3_BUCKET", "test-bucket")
	t.Setenv("ALLOW_ALL_UPSTREAM_GATEWAYS", "true")
	t.Setenv("ALLOW_ALL_SOURCE_BUCKETS", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.OriginSecrets) != 0 {
		t.Fatalf("OriginSecrets = %v, want empty", cfg.OriginSecrets)
	}
}

func TestLoadCSV_Single(t *testing.T) {
	t.Setenv("CF_ORIGIN_SECRET", "mysecret")
	t.Setenv("CACHE_S3_BUCKET", "test-bucket")
	t.Setenv("ALLOW_ALL_UPSTREAM_GATEWAYS", "true")
	t.Setenv("ALLOW_ALL_SOURCE_BUCKETS", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.OriginSecrets) != 1 || cfg.OriginSecrets[0] != "mysecret" {
		t.Fatalf("OriginSecrets = %v, want [mysecret]", cfg.OriginSecrets)
	}
}

func TestLoadCSV_Multiple(t *testing.T) {
	t.Setenv("CF_ORIGIN_SECRET", "new-secret,old-secret")
	t.Setenv("CACHE_S3_BUCKET", "test-bucket")
	t.Setenv("ALLOW_ALL_UPSTREAM_GATEWAYS", "true")
	t.Setenv("ALLOW_ALL_SOURCE_BUCKETS", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.OriginSecrets) != 2 {
		t.Fatalf("OriginSecrets len = %d, want 2", len(cfg.OriginSecrets))
	}
	if cfg.OriginSecrets[0] != "new-secret" || cfg.OriginSecrets[1] != "old-secret" {
		t.Fatalf("OriginSecrets = %v, want [new-secret old-secret]", cfg.OriginSecrets)
	}
}

func TestLoadCSV_AllowedGateways(t *testing.T) {
	t.Setenv("ALLOWED_UPSTREAM_GATEWAYS", "api.example.com,cdn.example.com")
	t.Setenv("CACHE_S3_BUCKET", "test-bucket")
	t.Setenv("ALLOW_ALL_SOURCE_BUCKETS", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.AllowedUpstreamGateways) != 2 {
		t.Fatalf("AllowedUpstreamGateways len = %d, want 2", len(cfg.AllowedUpstreamGateways))
	}
	if cfg.AllowedUpstreamGateways[0] != "api.example.com" || cfg.AllowedUpstreamGateways[1] != "cdn.example.com" {
		t.Fatalf("AllowedUpstreamGateways = %v, want [api.example.com cdn.example.com]", cfg.AllowedUpstreamGateways)
	}
}

func TestLoadCSV_TrimsWhitespace(t *testing.T) {
	t.Setenv("CF_ORIGIN_SECRET", " secret-a , secret-b ")
	t.Setenv("CACHE_S3_BUCKET", "test-bucket")
	t.Setenv("ALLOW_ALL_UPSTREAM_GATEWAYS", "true")
	t.Setenv("ALLOW_ALL_SOURCE_BUCKETS", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.OriginSecrets) != 2 {
		t.Fatalf("OriginSecrets len = %d, want 2", len(cfg.OriginSecrets))
	}
	if cfg.OriginSecrets[0] != "secret-a" || cfg.OriginSecrets[1] != "secret-b" {
		t.Fatalf("OriginSecrets = %v, want trimmed values", cfg.OriginSecrets)
	}
}

func TestAllowlistValidation(t *testing.T) {
	tests := []struct {
		name                     string
		allowedUpstreamGateways  string
		allowAllUpstreamGateways string
		allowedSourceBuckets     string
		allowAllSourceBuckets    string
		wantErr                  bool
	}{
		{
			name:                    "both allowlists set, no opt-in needed",
			allowedUpstreamGateways: "api.example.com",
			allowedSourceBuckets:    "my-bucket",
			wantErr:                 false,
		},
		{
			name:                     "both empty, both opt-in true",
			allowAllUpstreamGateways: "true",
			allowAllSourceBuckets:    "true",
			wantErr:                  false,
		},
		{
			name:                     "gateways set and opt-in true",
			allowedUpstreamGateways:  "api.example.com",
			allowAllUpstreamGateways: "true",
			allowedSourceBuckets:     "my-bucket",
			wantErr:                  false,
		},
		{
			name:                    "buckets set and opt-in true",
			allowedUpstreamGateways: "api.example.com",
			allowedSourceBuckets:    "my-bucket",
			allowAllSourceBuckets:   "true",
			wantErr:                 false,
		},
		{
			name:                  "gateways empty, opt-in false",
			allowedSourceBuckets:  "my-bucket",
			wantErr:               true,
		},
		{
			name:                     "gateways empty, opt-in true",
			allowAllUpstreamGateways: "true",
			allowedSourceBuckets:     "my-bucket",
			wantErr:                  false,
		},
		{
			name:                    "buckets empty, opt-in false",
			allowedUpstreamGateways: "api.example.com",
			wantErr:                 true,
		},
		{
			name:                    "buckets empty, opt-in true",
			allowedUpstreamGateways: "api.example.com",
			allowAllSourceBuckets:   "true",
			wantErr:                 false,
		},
		{
			name:    "both empty, both opt-in false",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CACHE_S3_BUCKET", "test-bucket")
			t.Setenv("ALLOWED_UPSTREAM_GATEWAYS", tc.allowedUpstreamGateways)
			t.Setenv("ALLOW_ALL_UPSTREAM_GATEWAYS", tc.allowAllUpstreamGateways)
			t.Setenv("ALLOWED_SOURCE_BUCKETS", tc.allowedSourceBuckets)
			t.Setenv("ALLOW_ALL_SOURCE_BUCKETS", tc.allowAllSourceBuckets)

			_, err := Load()
			if (err != nil) != tc.wantErr {
				t.Fatalf("Load() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

func TestAsyncCachePutConcurrencyZeroRejected(t *testing.T) {
	t.Setenv("CACHE_S3_BUCKET", "test-bucket")
	t.Setenv("ASYNC_CACHE_PUT_CONCURRENCY", "0")
	t.Setenv("ALLOW_ALL_UPSTREAM_GATEWAYS", "true")
	t.Setenv("ALLOW_ALL_SOURCE_BUCKETS", "true")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error for ASYNC_CACHE_PUT_CONCURRENCY=0")
	}
}

func TestAsyncCachePutTimeoutZeroRejected(t *testing.T) {
	t.Setenv("CACHE_S3_BUCKET", "test-bucket")
	t.Setenv("ASYNC_CACHE_PUT_TIMEOUT_SECONDS", "0")
	t.Setenv("ALLOW_ALL_UPSTREAM_GATEWAYS", "true")
	t.Setenv("ALLOW_ALL_SOURCE_BUCKETS", "true")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error for ASYNC_CACHE_PUT_TIMEOUT_SECONDS=0")
	}
}

func TestAsyncCachePutCustomValues(t *testing.T) {
	t.Setenv("CACHE_S3_BUCKET", "test-bucket")
	t.Setenv("ASYNC_CACHE_PUT_CONCURRENCY", "8")
	t.Setenv("ASYNC_CACHE_PUT_TIMEOUT_SECONDS", "60")
	t.Setenv("ALLOW_ALL_UPSTREAM_GATEWAYS", "true")
	t.Setenv("ALLOW_ALL_SOURCE_BUCKETS", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AsyncCachePutConcurrency != 8 {
		t.Fatalf("AsyncCachePutConcurrency = %d, want 8", cfg.AsyncCachePutConcurrency)
	}
	if cfg.AsyncCachePutTimeout != 60*time.Second {
		t.Fatalf("AsyncCachePutTimeout = %v, want 60s", cfg.AsyncCachePutTimeout)
	}
}

func TestLoadInvalidSettings(t *testing.T) {
	tests := []struct {
		name   string
		envVar string
		value  string
	}{
		// MAX_WIDTH invalid values
		{"MAX_WIDTH=0", "MAX_WIDTH", "0"},
		{"MAX_WIDTH=-1", "MAX_WIDTH", "-1"},
		{"MAX_WIDTH=abc", "MAX_WIDTH", "abc"},
		// MAX_BODY_BYTES invalid values
		{"MAX_BODY_BYTES=0", "MAX_BODY_BYTES", "0"},
		{"MAX_BODY_BYTES=-5", "MAX_BODY_BYTES", "-5"},
		// UPSTREAM_TIMEOUT invalid values
		{"UPSTREAM_TIMEOUT=0", "UPSTREAM_TIMEOUT", "0"},
		{"UPSTREAM_TIMEOUT=-1", "UPSTREAM_TIMEOUT", "-1"},
		{"UPSTREAM_TIMEOUT=abc", "UPSTREAM_TIMEOUT", "abc"},
		// IMGPROXY_TIMEOUT invalid values
		{"IMGPROXY_TIMEOUT=0", "IMGPROXY_TIMEOUT", "0"},
		// SHUTDOWN_TIMEOUT invalid values
		{"SHUTDOWN_TIMEOUT=0", "SHUTDOWN_TIMEOUT", "0"},
		// ASYNC_CACHE_PUT_CONCURRENCY invalid values
		{"ASYNC_CACHE_PUT_CONCURRENCY=0", "ASYNC_CACHE_PUT_CONCURRENCY", "0"},
		{"ASYNC_CACHE_PUT_CONCURRENCY=-1", "ASYNC_CACHE_PUT_CONCURRENCY", "-1"},
		// ASYNC_CACHE_PUT_TIMEOUT_SECONDS invalid values
		{"ASYNC_CACHE_PUT_TIMEOUT_SECONDS=0", "ASYNC_CACHE_PUT_TIMEOUT_SECONDS", "0"},
		// S3_MULTIPART_THRESHOLD_BYTES invalid values
		{"S3_MULTIPART_THRESHOLD_BYTES=0", "S3_MULTIPART_THRESHOLD_BYTES", "0"},
		{"S3_MULTIPART_THRESHOLD_BYTES=-1", "S3_MULTIPART_THRESHOLD_BYTES", "-1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CACHE_S3_BUCKET", "test")
			t.Setenv("ALLOW_ALL_UPSTREAM_GATEWAYS", "true")
			t.Setenv("ALLOW_ALL_SOURCE_BUCKETS", "true")
			t.Setenv(tc.envVar, tc.value)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() error = nil, want error for %s=%s", tc.envVar, tc.value)
			}
		})
	}
}

func TestLoadValidSettings(t *testing.T) {
	tests := []struct {
		name                       string
		maxWidth                   string
		maxBodyBytes               string
		upstreamTimeout            string
		asyncCachePutConcurrency   string
		wantMaxWidth               int
		wantMaxBodyBytes           int64
		wantUpstreamTimeout        time.Duration
		wantAsyncCachePutConcurrency int
	}{
		{
			name:                       "custom valid values",
			maxWidth:                   "640",
			maxBodyBytes:               "1048576",
			upstreamTimeout:            "10",
			asyncCachePutConcurrency:   "16",
			wantMaxWidth:               640,
			wantMaxBodyBytes:           1048576,
			wantUpstreamTimeout:        10 * time.Second,
			wantAsyncCachePutConcurrency: 16,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CACHE_S3_BUCKET", "test")
			t.Setenv("ALLOW_ALL_UPSTREAM_GATEWAYS", "true")
			t.Setenv("ALLOW_ALL_SOURCE_BUCKETS", "true")
			t.Setenv("MAX_WIDTH", tc.maxWidth)
			t.Setenv("MAX_BODY_BYTES", tc.maxBodyBytes)
			t.Setenv("UPSTREAM_TIMEOUT", tc.upstreamTimeout)
			t.Setenv("ASYNC_CACHE_PUT_CONCURRENCY", tc.asyncCachePutConcurrency)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.MaxWidth != tc.wantMaxWidth {
				t.Fatalf("MaxWidth = %d, want %d", cfg.MaxWidth, tc.wantMaxWidth)
			}
			if cfg.MaxBodyBytes != tc.wantMaxBodyBytes {
				t.Fatalf("MaxBodyBytes = %d, want %d", cfg.MaxBodyBytes, tc.wantMaxBodyBytes)
			}
			if cfg.UpstreamTimeout != tc.wantUpstreamTimeout {
				t.Fatalf("UpstreamTimeout = %v, want %v", cfg.UpstreamTimeout, tc.wantUpstreamTimeout)
			}
			if cfg.AsyncCachePutConcurrency != tc.wantAsyncCachePutConcurrency {
				t.Fatalf("AsyncCachePutConcurrency = %d, want %d", cfg.AsyncCachePutConcurrency, tc.wantAsyncCachePutConcurrency)
			}
		})
	}
}

func TestInvalidImgproxyURL(t *testing.T) {
	tests := []struct {
		name         string
		imgproxyURL  string
		shouldFail   bool
	}{
		{"valid http", "http://localhost:8081", false},
		{"valid https", "https://imgproxy.example.com", false},
		{"invalid ftp scheme", "ftp://host", true},
		{"invalid no scheme", "localhost:8081", true},
		{"invalid malformed", "not-a-url", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CACHE_S3_BUCKET", "test-bucket")
			t.Setenv("ALLOW_ALL_UPSTREAM_GATEWAYS", "true")
			t.Setenv("ALLOW_ALL_SOURCE_BUCKETS", "true")
			t.Setenv("IMGPROXY_URL", tc.imgproxyURL)

			_, err := Load()
			if tc.shouldFail && err == nil {
				t.Fatalf("Load() error = nil, want error for IMGPROXY_URL=%q", tc.imgproxyURL)
			}
			if !tc.shouldFail && err != nil {
				t.Fatalf("Load() error = %v, want nil for IMGPROXY_URL=%q", err, tc.imgproxyURL)
			}
		})
	}
}

