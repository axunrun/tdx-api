package main

import (
	"sync"
	"time"
)

type agentTTLCache[T any] struct {
	mu       sync.Mutex
	loader   func() (T, error)
	ttl      time.Duration
	value    T
	loadedAt time.Time
	loaded   bool
}

func newAgentTTLCache[T any](loader func() (T, error), ttl time.Duration) *agentTTLCache[T] {
	return &agentTTLCache[T]{
		loader: loader,
		ttl:    ttl,
	}
}

func (c *agentTTLCache[T]) Get() (T, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.loaded && time.Since(c.loadedAt) < c.ttl {
		return c.value, nil
	}
	value, err := c.loader()
	if err != nil {
		var zero T
		return zero, err
	}
	c.value = value
	c.loadedAt = time.Now()
	c.loaded = true
	return value, nil
}
