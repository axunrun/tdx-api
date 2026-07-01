package main

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestStartBackgroundInitializersReturnsBeforeTasksFinish(t *testing.T) {
	var started int32
	release := make(chan struct{})
	done := make(chan struct{})

	startBackgroundInitializers(
		func() {
			close(done)
		},
		func() {
			atomic.AddInt32(&started, 1)
			<-release
		},
		func() {
			atomic.AddInt32(&started, 1)
			<-release
		},
	)

	deadline := time.After(100 * time.Millisecond)
	for atomic.LoadInt32(&started) < 2 {
		select {
		case <-deadline:
			t.Fatal("background initializers did not start in time")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	select {
	case <-done:
		t.Fatal("completion callback ran before tasks were released")
	default:
	}

	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("completion callback did not run after tasks finished")
	}
}
