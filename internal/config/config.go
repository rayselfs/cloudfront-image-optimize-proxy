package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	defaultListenAddr    = ":9999"
	defaultImgproxyURL   = "http://localhost:8081"
	defaultCacheS3Region = "us-west-2"
	defaultMaxWidth      = 1920
)

// Config holds the service configuration loaded from environment variables.
type Config struct {
	ListenAddr    string
	ImgproxyURL   string
	CacheS3Bucket string
	CacheS3Region string
	MaxWidth      int
}

// Load reads service configuration from environment variables.
func Load() (*Config, error) {
	maxWidth, err := loadMaxWidth()
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		ListenAddr:    envOrDefault("LISTEN_ADDR", defaultListenAddr),
		ImgproxyURL:   envOrDefault("IMGPROXY_URL", defaultImgproxyURL),
		CacheS3Bucket: strings.TrimSpace(os.Getenv("CACHE_S3_BUCKET")),
		CacheS3Region: envOrDefault("CACHE_S3_REGION", defaultCacheS3Region),
		MaxWidth:      maxWidth,
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

func loadMaxWidth() (int, error) {
	value := strings.TrimSpace(os.Getenv("MAX_WIDTH"))
	if value == "" {
		return defaultMaxWidth, nil
	}

	maxWidth, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("MAX_WIDTH must be an integer: %w", err)
	}
	if maxWidth <= 0 {
		return 0, fmt.Errorf("MAX_WIDTH must be greater than zero")
	}

	return maxWidth, nil
}
