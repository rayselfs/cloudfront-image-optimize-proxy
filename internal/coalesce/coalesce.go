package coalesce

import "golang.org/x/sync/singleflight"

// Coalescer deduplicates concurrent requests for the same resource.
type Coalescer interface {
	Do(key string, fn func() (interface{}, error)) (interface{}, error, bool)
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
// Returns (value, error, shared) where shared=true means the result was
// shared with another concurrent caller.
func (s *SingleFlight) Do(key string, fn func() (interface{}, error)) (interface{}, error, bool) {
	val, err, shared := s.group.Do(key, func() (interface{}, error) {
		return fn()
	})
	return val, err, shared
}
