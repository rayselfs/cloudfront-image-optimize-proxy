package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultListenAddr              = ":9999"
	defaultImgproxyURL             = "http://localhost:8081"
	defaultCacheS3Region           = "us-west-2"
	defaultMaxWidth                = 1920
	defaultMaxBodyBytes            = 20 * 1024 * 1024 // 20 MB
	defaultUpstreamTimeout         = 30 * time.Second
	defaultImgproxyTimeout         = 30 * time.Second
	defaultShutdownTimeout         = 25 * time.Second
	defaultAsyncCachePutConcurrency = 32
	defaultAsyncCachePutTimeout     = 30 * time.Second
	defaultMultipartThresholdBytes int64 = 5 * 1024 * 1024 // 5 MiB
)

// Config holds the service configuration loaded from environment variables.
type Config struct {
	ListenAddr                  string
	ImgproxyURL                 string
	CacheS3Bucket               string
	CacheS3Region               string
	MaxWidth                    int
	MaxBodyBytes                int64
	UpstreamTimeout             time.Duration
	ImgproxyTimeout             time.Duration
	ShutdownTimeout             time.Duration
	AsyncCachePutConcurrency    int
	AsyncCachePutTimeout        time.Duration
	OriginSecrets               []string
	AllowedUpstreamGateways     []string
	AllowedSourceBuckets        []string
	AllowAllUpstreamGateways    bool
	AllowAllSourceBuckets       bool
	MultipartThresholdBytes     int64
	S3ReadinessCheckEnabled     bool
}

// Load reads service configuration from environment variables.
func Load() (*Config, error) {
	maxWidth, err := loadPositiveInt("MAX_WIDTH", defaultMaxWidth)
	if err != nil {
		return nil, err
	}

	maxBodyBytes, err := loadPositiveInt64("MAX_BODY_BYTES", defaultMaxBodyBytes)
	if err != nil {
		return nil, err
	}

	upstreamTimeout, err := loadDurationSeconds("UPSTREAM_TIMEOUT", defaultUpstreamTimeout)
	if err != nil {
		return nil, err
	}

	imgproxyTimeout, err := loadDurationSeconds("IMGPROXY_TIMEOUT", defaultImgproxyTimeout)
	if err != nil {
		return nil, err
	}

	shutdownTimeout, err := loadDurationSeconds("SHUTDOWN_TIMEOUT", defaultShutdownTimeout)
	if err != nil {
		return nil, err
	}

	asyncCachePutConcurrency, err := loadPositiveInt("ASYNC_CACHE_PUT_CONCURRENCY", defaultAsyncCachePutConcurrency)
	if err != nil {
		return nil, err
	}

	asyncCachePutTimeout, err := loadDurationSeconds("ASYNC_CACHE_PUT_TIMEOUT_SECONDS", defaultAsyncCachePutTimeout)
	if err != nil {
		return nil, err
	}

	multipartThreshold, err := loadPositiveInt64("S3_MULTIPART_THRESHOLD_BYTES", defaultMultipartThresholdBytes)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		ListenAddr:                  envOrDefault("LISTEN_ADDR", defaultListenAddr),
		ImgproxyURL:                 envOrDefault("IMGPROXY_URL", defaultImgproxyURL),
		CacheS3Bucket:               strings.TrimSpace(os.Getenv("CACHE_S3_BUCKET")),
		CacheS3Region:               envOrDefault("CACHE_S3_REGION", defaultCacheS3Region),
		MaxWidth:                    maxWidth,
		MaxBodyBytes:                maxBodyBytes,
		UpstreamTimeout:             upstreamTimeout,
		ImgproxyTimeout:             imgproxyTimeout,
		ShutdownTimeout:             shutdownTimeout,
		AsyncCachePutConcurrency:    asyncCachePutConcurrency,
		AsyncCachePutTimeout:        asyncCachePutTimeout,
		OriginSecrets:               loadCSV("CF_ORIGIN_SECRET"),
		AllowedUpstreamGateways:     loadCSV("ALLOWED_UPSTREAM_GATEWAYS"),
		AllowedSourceBuckets:        loadCSV("ALLOWED_SOURCE_BUCKETS"),
		AllowAllUpstreamGateways:    loadBool("ALLOW_ALL_UPSTREAM_GATEWAYS"),
		AllowAllSourceBuckets:       loadBool("ALLOW_ALL_SOURCE_BUCKETS"),
		MultipartThresholdBytes:     multipartThreshold,
		S3ReadinessCheckEnabled:     loadBool("S3_READINESS_CHECK_ENABLED"),
	}

	if cfg.CacheS3Bucket == "" {
		return nil, fmt.Errorf("CACHE_S3_BUCKET is required")
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func envOrDefault(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func loadPositiveInt(name string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}

	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", name, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", name)
	}

	return n, nil
}

func loadPositiveInt64(name string, fallback int64) (int64, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}

	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", name, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", name)
	}

	return n, nil
}

func loadDurationSeconds(name string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}

	secs, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer (seconds): %w", name, err)
	}
	if secs <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", name)
	}

	return time.Duration(secs) * time.Second, nil
}

func loadCSV(name string) []string {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func loadBool(name string) bool {
	return strings.ToLower(strings.TrimSpace(os.Getenv(name))) == "true"
}

// Validate checks that allowlists are configured or explicitly opted out,
// and validates ImgproxyURL is a valid HTTP URL.
func (c *Config) Validate() error {
	if len(c.AllowedUpstreamGateways) == 0 && !c.AllowAllUpstreamGateways {
		return fmt.Errorf("ALLOWED_UPSTREAM_GATEWAYS is empty; set it or set ALLOW_ALL_UPSTREAM_GATEWAYS=true to permit all")
	}
	if len(c.AllowedSourceBuckets) == 0 && !c.AllowAllSourceBuckets {
		return fmt.Errorf("ALLOWED_SOURCE_BUCKETS is empty; set it or set ALLOW_ALL_SOURCE_BUCKETS=true to permit all")
	}

	if _, err := url.ParseRequestURI(c.ImgproxyURL); err != nil || !strings.HasPrefix(c.ImgproxyURL, "http") {
		return fmt.Errorf("IMGPROXY_URL %q is not a valid HTTP URL", c.ImgproxyURL)
	}

	return nil
}
