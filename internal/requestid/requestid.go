package requestid

import "context"

type contextKey struct{}

// key is the unexported context key for the request ID.
var key = contextKey{}

// FromContext returns the request ID stored in ctx, or empty string if absent.
func FromContext(ctx context.Context) string {
	if id, ok := ctx.Value(key).(string); ok {
		return id
	}
	return ""
}

// WithContext returns a new context carrying the given request ID.
func WithContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, key, id)
}
