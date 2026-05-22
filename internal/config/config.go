package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultListenAddr      = ":9999"
	defaultImgproxyURL     = "http://localhost:8081"
	defaultCacheS3Region   = "us-west-2"
	defaultMaxWidth        = 1920
	defaultUpstreamTimeout = 30 * time.Second
	defaultImgproxyTimeout = 30 * time.Second
)

// Config holds the service configuration loaded from environment variables.
type Config struct {
	ListenAddr      string
	ImgproxyURL     string
	CacheS3Bucket   string
	CacheS3Region   string
	MaxWidth        int
	UpstreamTimeout time.Duration
	ImgproxyTimeout time.Duration
}

// Load reads service configuration from environment variables.
func Load() (*Config, error) {
	maxWidth, err := loadPositiveInt("MAX_WIDTH", defaultMaxWidth)
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

	cfg := &Config{
		ListenAddr:      envOrDefault("LISTEN_ADDR", defaultListenAddr),
		ImgproxyURL:     envOrDefault("IMGPROXY_URL", defaultImgproxyURL),
		CacheS3Bucket:   strings.TrimSpace(os.Getenv("CACHE_S3_BUCKET")),
		CacheS3Region:   envOrDefault("CACHE_S3_REGION", defaultCacheS3Region),
		MaxWidth:        maxWidth,
		UpstreamTimeout: upstreamTimeout,
		ImgproxyTimeout: imgproxyTimeout,
	}

	if cfg.CacheS3Bucket == "" {
		return nil, fmt.Errorf("CACHE_S3_BUCKET is required")
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
