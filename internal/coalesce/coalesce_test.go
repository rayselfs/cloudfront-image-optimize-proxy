package coalesce

import (
	"context"
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
	errs := make([]error, numGoroutines)
	shared := make([]bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-ready
			results[idx], errs[idx], shared[idx] = c.Do(context.Background(), "same-key", fn)
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
		if errs[i] != nil {
			t.Errorf("goroutine %d: expected no error, got %v", i, errs[i])
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
		result1, err1, _ = c.Do(context.Background(), "key1", fn1)
	}()

	go func() {
		defer wg.Done()
		<-ready
		result2, err2, _ = c.Do(context.Background(), "key2", fn2)
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
	errs := make([]error, 3)

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-ready
			_, errs[idx], _ = c.Do(context.Background(), "error-key", fn)
		}(i)
	}

	close(ready)
	wg.Wait()

	for i := 0; i < 3; i++ {
		if errs[i] != testErr {
			t.Errorf("goroutine %d: expected error %v, got %v", i, testErr, errs[i])
		}
	}
}

// TestContextCancellation verifies that a cancelled context returns ctx.Err()
// without waiting for the underlying function to complete.
func TestContextCancellation(t *testing.T) {
	c := New()

	block := make(chan struct{})
	fn := func() (interface{}, error) {
		<-block
		return "result", nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	_, err, _ := c.Do(ctx, "cancel-key", fn)
	close(block) // unblock any goroutine that may have started

	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
