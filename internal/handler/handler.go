package handler

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/rayselfs/cloudfront-image-optimize-proxy/internal/cache"
	"github.com/rayselfs/cloudfront-image-optimize-proxy/internal/coalesce"
	"github.com/rayselfs/cloudfront-image-optimize-proxy/internal/imgproxy"
	"github.com/rayselfs/cloudfront-image-optimize-proxy/internal/metrics"
	"github.com/rayselfs/cloudfront-image-optimize-proxy/internal/upstream"
)

// Handler is the main image optimization HTTP handler.
type Handler struct {
	Cache        cache.Cache
	Transformer  imgproxy.Transformer
	Resolver     upstream.Resolver
	Coalescer    coalesce.Coalescer
	MaxWidth     int
	MaxBodyBytes int64
}

type processResult struct {
	body        []byte
	contentType string
	cacheStatus string
}

// New creates a new Handler. maxBodyBytes limits upstream/transform body reads (0 = no limit).
func New(c cache.Cache, t imgproxy.Transformer, r upstream.Resolver, coal coalesce.Coalescer, maxWidth int, maxBodyBytes int64) *Handler {
	return &Handler{
		Cache:        c,
		Transformer:  t,
		Resolver:     r,
		Coalescer:    coal,
		MaxWidth:     maxWidth,
		MaxBodyBytes: maxBodyBytes,
	}
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	params := ParseParams(r.URL.Query())
	if params == nil {
		h.passThrough(w, r)
		return
	}

	if h.MaxWidth > 0 && params.Width > h.MaxWidth {
		params.Width = h.MaxWidth
	}

	key := cache.KeyFromRequest(r.Host, r.URL.Path, cache.ImageParams{
		Width:   params.Width,
		Format:  params.Format,
		Quality: params.Quality,
	})

	value, err, _ := h.Coalescer.Do(r.Context(), key, func() (interface{}, error) {
		return h.process(r, key, params)
	})
	if err != nil {
		slog.Error("handler: process request", "error", err)
		h.writeError(w)
		return
	}

	result, ok := value.(processResult)
	if !ok {
		slog.Error("handler: unexpected coalescer result type", "type", fmt.Sprintf("%T", value))
		h.writeError(w)
		return
	}
	h.writeResult(w, result)
}

func (h *Handler) passThrough(w http.ResponseWriter, r *http.Request) {
	_, fetchFunc, err := h.Resolver.Resolve(r)
	if err != nil {
		slog.Error("handler: resolve pass-through", "error", err)
		h.writeError(w)
		return
	}

	body, contentType, err := fetchFunc()
	if err != nil {
		slog.Error("handler: fetch pass-through", "error", err)
		h.writeError(w)
		return
	}
	defer body.Close()

	if h.MaxBodyBytes <= 0 {
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		if _, err := io.Copy(w, body); err != nil {
			slog.Error("handler: write pass-through", "error", err)
		}
		return
	}

	limited := io.LimitReader(body, h.MaxBodyBytes+1)
	buf, err := io.ReadAll(limited)
	if err != nil {
		slog.Error("handler: read pass-through body", "error", err)
		h.writeError(w)
		return
	}
	if int64(len(buf)) > h.MaxBodyBytes {
		http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
		return
	}
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	_, _ = w.Write(buf)
}

func (h *Handler) process(r *http.Request, key string, params *ImageParams) (processResult, error) {
	if body, contentType, err := h.Cache.Get(r.Context(), key); err == nil {
		defer body.Close()
		data, err := h.readBody(body)
		if err != nil {
			return processResult{}, err
		}
		metrics.IncCacheHit()
		return processResult{body: data, contentType: contentType, cacheStatus: "HIT"}, nil
	} else if !errors.Is(err, cache.ErrNotFound) {
		slog.Error("handler: cache get", "key", key, "error", err)
	}

	sourceURL, fetchFunc, err := h.Resolver.Resolve(r)
	if err != nil {
		return processResult{}, err
	}

	originalBody, originalContentType, err := fetchFunc()
	if err != nil {
		return processResult{}, err
	}
	defer originalBody.Close()

	originalData, err := h.readBody(originalBody)
	if err != nil {
		return processResult{}, err
	}

	if !strings.HasPrefix(originalContentType, "image/") {
		metrics.IncCacheBypass()
		return processResult{body: originalData, contentType: originalContentType, cacheStatus: "BYPASS"}, nil
	}

	transformedBody, transformedContentType, err := h.Transformer.Transform(r.Context(), sourceURL, imgproxy.TransformParams{
		Width:   params.Width,
		Format:  params.Format,
		Quality: params.Quality,
	})
	if err != nil {
		slog.Error("handler: transform failed, using original",
			"error", err,
			"path", r.URL.Path,
		)
		metrics.IncImgproxyError()
		metrics.IncCacheMiss()
		return processResult{body: originalData, contentType: originalContentType, cacheStatus: "MISS"}, nil
	}
	defer transformedBody.Close()

	data, err := h.readBody(transformedBody)
	if err != nil {
		return processResult{}, err
	}
	if err := h.Cache.Put(r.Context(), key, bytes.NewReader(data), transformedContentType); err != nil {
		slog.Error("handler: cache put", "key", key, "error", err)
	}

	metrics.IncCacheMiss()
	return processResult{body: data, contentType: transformedContentType, cacheStatus: "MISS"}, nil
}

func (h *Handler) readBody(r io.Reader) ([]byte, error) {
	if h.MaxBodyBytes <= 0 {
		return io.ReadAll(r)
	}
	limit := h.MaxBodyBytes + 1
	data, err := io.ReadAll(io.LimitReader(r, limit))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > h.MaxBodyBytes {
		return nil, fmt.Errorf("response body exceeds %d bytes", h.MaxBodyBytes)
	}
	return data, nil
}

func (h *Handler) writeResult(w http.ResponseWriter, result processResult) {
	if result.contentType != "" {
		w.Header().Set("Content-Type", result.contentType)
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	w.Header().Set("X-Cache", result.cacheStatus)
	_, _ = w.Write(result.body)
}

func (h *Handler) writeError(w http.ResponseWriter) {
	http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
}
