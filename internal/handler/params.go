package handler

import (
	"net/url"
	"strconv"
)

// ImageParams holds request image optimization parameters.
type ImageParams struct {
	Width   int
	Format  string
	Quality int
}

// ParseParams extracts imwidth, f, q from query string.
// Returns nil if no optimize params are present or imwidth is invalid.
func ParseParams(query url.Values, defaultQuality int) *ImageParams {
	widthValue := query.Get("imwidth")
	if widthValue == "" {
		return nil
	}

	width, err := strconv.Atoi(widthValue)
	if err != nil || width <= 0 {
		return nil
	}

	format := query.Get("f")
	if !isSupportedFormat(format) {
		return nil
	}

	quality := defaultQuality
	if qualityValue := query.Get("q"); qualityValue != "" {
		parsedQuality, err := strconv.Atoi(qualityValue)
		if err == nil {
			quality = parsedQuality
		}
	}

	if quality < 1 {
		quality = 1
	}
	if quality > 100 {
		quality = 100
	}

	return &ImageParams{
		Width:   width,
		Format:  format,
		Quality: quality,
	}
}

func isSupportedFormat(format string) bool {
	switch format {
	case "webp", "avif", "jpeg":
		return true
	default:
		return false
	}
}
