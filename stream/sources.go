package stream

import (
	"iter"
	"math"
)

// Empty returns an empty reusable Stream in O(1) time and memory.
func Empty[T any]() Stream[T] {
	return Stream[T]{}
}

// Of makes a shallow snapshot of values and returns a reusable Stream over the
// snapshot. Construction takes O(n) time and memory; traversal is lazy,
// ordered, and uses O(1) state.
func Of[T any](values ...T) Stream[T] {
	snapshot := make([]T, len(values))
	copy(snapshot, values)
	return FromSlice(snapshot)
}

// FromSlice returns a reusable zero-copy view of the captured slice range.
// Element replacements are observed by later traversals. Construction uses
// O(1) time and memory, and traversal preserves encounter order.
func FromSlice[T any](values []T) Stream[T] {
	return Stream[T]{seq: func(yield func(T) bool) {
		for _, value := range values {
			if !yield(value) {
				return
			}
		}
	}}
}

// FromSeq wraps seq without caching or changing its replay behavior. A nil seq
// is normalized to an empty Stream. It preserves order, early termination,
// cleanup, and reusable or single-use behavior in O(1) wrapper state.
func FromSeq[T any](seq iter.Seq[T]) Stream[T] {
	if seq == nil {
		return Empty[T]()
	}
	return Stream[T]{seq: seq}
}

// FromSeq2 lazily converts each two-value iterator element into a Pair. A nil
// seq is empty. Conversion is lazy, ordered, and uses O(1) wrapper state with
// no Pair-specific per-element heap allocation.
func FromSeq2[A, B any](seq iter.Seq2[A, B]) Stream[Pair[A, B]] {
	if seq == nil {
		return Empty[Pair[A, B]]()
	}
	return Stream[Pair[A, B]]{seq: func(yield func(Pair[A, B]) bool) {
		seq(func(a A, b B) bool {
			return yield(Pair[A, B]{First: a, Second: b})
		})
	}}
}

// ToSeq2 lazily converts a Stream of Pair values into a non-nil two-value
// iterator sequence. It preserves order, early termination, and source replay
// behavior with O(1) wrapper state.
func ToSeq2[A, B any](s Stream[Pair[A, B]]) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		s.Seq()(func(pair Pair[A, B]) bool {
			return yield(pair.First, pair.Second)
		})
	}
}

// Range returns the reusable half-open ascending sequence from start to end.
// It is empty when start is not less than end and terminates before overflow.
// Traversal takes O(n) time with O(1) state.
func Range(start, end int) Stream[int] {
	return RangeStep(start, end, 1)
}

// RangeStep returns a reusable half-open arithmetic sequence. It panics
// immediately when step is zero and terminates instead of overflowing int.
// Traversal takes O(n) time with O(1) state.
func RangeStep(start, end, step int) Stream[int] {
	if step == 0 {
		panic("stream: RangeStep step must not be zero")
	}
	return Stream[int]{seq: func(yield func(int) bool) {
		if step > 0 {
			for current := start; current < end; {
				if !yield(current) {
					return
				}
				if current > math.MaxInt-step {
					return
				}
				current += step
			}
			return
		}

		for current := start; current > end; {
			if !yield(current) {
				return
			}
			if current < math.MinInt-step {
				return
			}
			current += step
		}
	}}
}

// Repeat returns an infinite reusable Stream of shallow copies of value. It is
// infinite-safe and uses O(1) traversal state.
func Repeat[T any](value T) Stream[T] {
	return Stream[T]{seq: func(yield func(T) bool) {
		for yield(value) {
		}
	}}
}

// RepeatN returns a reusable Stream containing n shallow copies of value. It
// panics immediately when n is negative. Traversal is lazy and uses O(1) state.
func RepeatN[T any](value T, n int) Stream[T] {
	if n < 0 {
		panic("stream: RepeatN count must not be negative")
	}
	return Stream[T]{seq: func(yield func(T) bool) {
		for range n {
			if !yield(value) {
				return
			}
		}
	}}
}

// Iterate returns an infinite Stream that starts with seed and applies next
// only after the preceding value is accepted and another value is requested.
// Each traversal has its own current value. Iterate is infinite-safe and uses
// O(1) traversal state; callback state remains caller-owned.
func Iterate[T any](seed T, next func(T) T) Stream[T] {
	return Stream[T]{seq: func(yield func(T) bool) {
		current := seed
		for {
			if !yield(current) {
				return
			}
			current = next(current)
		}
	}}
}

// Generate returns an infinite Stream that invokes next once immediately before
// every emitted value. Captured callback state is shared across traversals.
// Generate is infinite-safe and uses O(1) Shuttle state.
func Generate[T any](next func() T) Stream[T] {
	return Stream[T]{seq: func(yield func(T) bool) {
		for {
			if !yield(next()) {
				return
			}
		}
	}}
}
