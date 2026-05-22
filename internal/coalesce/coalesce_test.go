package coalesce

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestConcurrentDedup verifies that concurrent requests with the same key
// result in only one execution of the function.
func TestConcurrentDedup(t *testing.T) {
	c := New()
	var executionCount atomic.Int32
	const numGoroutines = 10

	// Use a channel to synchronize goroutine start
	ready := make(chan struct{})

	fn := func() (interface{}, error) {
		executionCount.Add(1)
		time.Sleep(1 * time.Millisecond)
		return "result", nil
	}

	var wg sync.WaitGroup
	results := make([]interface{}, numGoroutines)
	errors := make([]error, numGoroutines)
	shared := make([]bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-ready
			results[idx], errors[idx], shared[idx] = c.Do("same-key", fn)
		}(i)
	}

	close(ready)
	wg.Wait()

	// Verify only one execution happened
	if count := executionCount.Load(); count != 1 {
		t.Errorf("expected 1 execution, got %d", count)
	}

	// Verify all goroutines got the same result
	for i := 0; i < numGoroutines; i++ {
		if results[i] != "result" {
			t.Errorf("goroutine %d: expected result 'result', got %v", i, results[i])
		}
		if errors[i] != nil {
			t.Errorf("goroutine %d: expected no error, got %v", i, errors[i])
		}
	}

	// Verify all callers got shared=true (since there were multiple concurrent callers)
	for i := 0; i < numGoroutines; i++ {
		if !shared[i] {
			t.Errorf("goroutine %d should have shared=true, got false", i)
		}
	}
}

// TestDifferentKeys verifies that different keys execute independently.
func TestDifferentKeys(t *testing.T) {
	c := New()
	var count1, count2 atomic.Int32
	ready := make(chan struct{})

	fn1 := func() (interface{}, error) {
		count1.Add(1)
		time.Sleep(1 * time.Millisecond)
		return "result1", nil
	}

	fn2 := func() (interface{}, error) {
		count2.Add(1)
		time.Sleep(1 * time.Millisecond)
		return "result2", nil
	}

	var wg sync.WaitGroup
	var result1, result2 interface{}
	var err1, err2 error

	wg.Add(2)
	go func() {
		defer wg.Done()
		<-ready
		result1, err1, _ = c.Do("key1", fn1)
	}()

	go func() {
		defer wg.Done()
		<-ready
		result2, err2, _ = c.Do("key2", fn2)
	}()

	close(ready)
	wg.Wait()

	if count1.Load() != 1 {
		t.Errorf("expected 1 execution for key1, got %d", count1.Load())
	}
	if count2.Load() != 1 {
		t.Errorf("expected 1 execution for key2, got %d", count2.Load())
	}
	if result1 != "result1" {
		t.Errorf("expected result1, got %v", result1)
	}
	if result2 != "result2" {
		t.Errorf("expected result2, got %v", result2)
	}
	if err1 != nil {
		t.Errorf("expected no error for key1, got %v", err1)
	}
	if err2 != nil {
		t.Errorf("expected no error for key2, got %v", err2)
	}
}

// TestErrorPropagation verifies that errors are propagated to all waiters.
func TestErrorPropagation(t *testing.T) {
	c := New()
	testErr := errors.New("test error")
	ready := make(chan struct{})

	fn := func() (interface{}, error) {
		time.Sleep(1 * time.Millisecond)
		return nil, testErr
	}

	var wg sync.WaitGroup
	errors := make([]error, 3)

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-ready
			_, errors[idx], _ = c.Do("error-key", fn)
		}(i)
	}

	close(ready)
	wg.Wait()

	for i := 0; i < 3; i++ {
		if errors[i] != testErr {
			t.Errorf("goroutine %d: expected error %v, got %v", i, testErr, errors[i])
		}
	}
}
