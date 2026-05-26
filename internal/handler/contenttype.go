package handler

import (
	"fmt"
	"strings"
)

// validatePassThroughContentType checks that a content type from an upstream
// pass-through response does not contain control characters that could enable
// HTTP response header injection.
func validatePassThroughContentType(ct string) error {
	if strings.ContainsAny(ct, "\r\n") {
		return fmt.Errorf("content type contains illegal control characters")
	}
	return nil
}

// validateTransformedContentType checks that a content type returned by
// imgproxy is a valid image type and free of control characters.
func validateTransformedContentType(ct string) error {
	if strings.ContainsAny(ct, "\r\n") {
		return fmt.Errorf("content type contains illegal control characters")
	}
	if !strings.HasPrefix(ct, "image/") {
		return fmt.Errorf("transformed content type %q is not an image type", ct)
	}
	return nil
}
