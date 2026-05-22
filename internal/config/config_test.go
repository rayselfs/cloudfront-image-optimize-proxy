package config

import "testing"

func TestDefaultConfig(t *testing.T) {
	t.Setenv("LISTEN_ADDR", "")
	t.Setenv("IMGPROXY_URL", "")
	t.Setenv("CACHE_S3_BUCKET", "source-images")
	t.Setenv("CACHE_S3_REGION", "")
	t.Setenv("MAX_WIDTH", "")

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
