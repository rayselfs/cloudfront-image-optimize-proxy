package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rayselfs/cloudfront-image-optimize-proxy/internal/cache"
	"github.com/rayselfs/cloudfront-image-optimize-proxy/internal/coalesce"
	appconfig "github.com/rayselfs/cloudfront-image-optimize-proxy/internal/config"
	"github.com/rayselfs/cloudfront-image-optimize-proxy/internal/handler"
	"github.com/rayselfs/cloudfront-image-optimize-proxy/internal/imgproxy"
	"github.com/rayselfs/cloudfront-image-optimize-proxy/internal/metrics"
	"github.com/rayselfs/cloudfront-image-optimize-proxy/internal/middleware"
	"github.com/rayselfs/cloudfront-image-optimize-proxy/internal/upstream"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	cfg, err := appconfig.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	// Startup security warnings.
	if len(cfg.OriginSecrets) == 0 {
		slog.Warn("CF_ORIGIN_SECRET is not set; origin verification is disabled")
	}
	if len(cfg.AllowedUpstreamGateways) == 0 {
		slog.Warn("ALLOWED_UPSTREAM_GATEWAYS is not set; any upstream gateway is permitted")
	}
	if len(cfg.AllowedSourceBuckets) == 0 {
		slog.Warn("ALLOWED_SOURCE_BUCKETS is not set; any S3 source bucket is permitted")
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(cfg.CacheS3Region))
	if err != nil {
		slog.Error("load aws config", "error", err)
		os.Exit(1)
	}

	s3Cache := cache.NewS3Cache(s3.NewFromConfig(awsCfg), cfg.CacheS3Bucket)
	asyncCache := cache.WrapAsyncPut(s3Cache, 30*time.Second)
	imgproxyClient := imgproxy.NewClient(cfg.ImgproxyURL, cfg.ImgproxyTimeout)
	resolver := upstream.NewResolver(cfg.UpstreamTimeout, cfg.AllowedUpstreamGateways, cfg.AllowedSourceBuckets)
	coalescer := coalesce.New()
	imageHandler := handler.New(asyncCache, imgproxyClient, resolver, coalescer, cfg.MaxWidth, cfg.MaxBodyBytes)

	readyClient := &http.Client{Timeout: 2 * time.Second}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, cfg.ImgproxyURL+"/health", nil)
		if err != nil {
			http.Error(w, "imgproxy not ready", http.StatusServiceUnavailable)
			return
		}
		resp, err := readyClient.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil {
				resp.Body.Close()
			}
			http.Error(w, "imgproxy not ready", http.StatusServiceUnavailable)
			return
		}
		resp.Body.Close()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("GET /metrics", metrics.Handler())
	mux.Handle("/", imageHandler)

	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      middleware.Logging(middleware.CloudFrontVerify(cfg.OriginSecrets)(mux)),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("server starting", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}

	asyncCache.Wait()
	slog.Info("async cache drained")
}
