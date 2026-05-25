package coalesce

import (
	"context"

	"golang.org/x/sync/singleflight"
)

// Coalescer deduplicates concurrent requests for the same resource.
type Coalescer interface {
	Do(ctx context.Context, key string, fn func() (interface{}, error)) (interface{}, error, bool)
}

// SingleFlight implements Coalescer using golang.org/x/sync/singleflight.
type SingleFlight struct {
	group singleflight.Group
}

// New creates a new SingleFlight coalescer.
func New() *SingleFlight {
	return &SingleFlight{}
}

// Do executes fn for the given key, deduplicating concurrent calls.
// If ctx is cancelled before the result is ready, Do returns immediately with ctx.Err().
// The underlying fn continues running so the result can be shared with other callers.
func (s *SingleFlight) Do(ctx context.Context, key string, fn func() (interface{}, error)) (interface{}, error, bool) {
	ch := s.group.DoChan(key, func() (interface{}, error) {
		return fn()
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err(), false
	case result := <-ch:
		return result.Val, result.Err, result.Shared
	}
}
