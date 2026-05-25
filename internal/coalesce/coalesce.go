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
//
// NOTE: The underlying singleflight call intentionally continues after the caller's context is cancelled.
// All waiters sharing the same in-flight request will receive the result once it completes.
// Implementing all-callers-cancel would require interface churn and is not currently needed.
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
