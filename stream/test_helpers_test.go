package stream_test

import (
	"iter"
	"slices"
	"sync/atomic"
	"testing"
)

type sequenceProbe struct {
	starts   atomic.Int64
	consumed atomic.Int64
	cleanups atomic.Int64
}

func instrumentedSeq[T any](values []T, probe *sequenceProbe) iter.Seq[T] {
	return func(yield func(T) bool) {
		probe.starts.Add(1)
		defer probe.cleanups.Add(1)
		for _, value := range values {
			probe.consumed.Add(1)
			if !yield(value) {
				return
			}
		}
	}
}

func requireSliceEqual[T comparable](t *testing.T, got, want []T) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func requirePanics(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("%s did not panic", name)
		}
	}()
	fn()
}

func requirePanicValue(t *testing.T, want any, fn func()) {
	t.Helper()
	defer func() {
		if got := recover(); got != want {
			t.Fatalf("panic = %#v, want %#v", got, want)
		}
	}()
	fn()
}
