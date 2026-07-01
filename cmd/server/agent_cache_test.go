package main

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestCachedAgentValueUsesLoaderOnceWithinTTL(t *testing.T) {
	var calls int32
	cache := newAgentTTLCache(func() (int, error) {
		atomic.AddInt32(&calls, 1)
		return 42, nil
	}, time.Minute)

	first, err := cache.Get()
	if err != nil {
		t.Fatalf("first Get() error = %v", err)
	}
	second, err := cache.Get()
	if err != nil {
		t.Fatalf("second Get() error = %v", err)
	}
	if first != 42 || second != 42 {
		t.Fatalf("values = %d, %d; want both 42", first, second)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("loader calls = %d, want 1", got)
	}
}

func TestCachedAgentValueReloadsAfterTTL(t *testing.T) {
	var calls int32
	cache := newAgentTTLCache(func() (int, error) {
		return int(atomic.AddInt32(&calls, 1)), nil
	}, time.Nanosecond)

	first, err := cache.Get()
	if err != nil {
		t.Fatalf("first Get() error = %v", err)
	}
	time.Sleep(time.Millisecond)
	second, err := cache.Get()
	if err != nil {
		t.Fatalf("second Get() error = %v", err)
	}
	if first != 1 || second != 2 {
		t.Fatalf("values = %d, %d; want 1, 2", first, second)
	}
}
