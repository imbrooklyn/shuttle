package stream

import (
	"iter"
	"math"

	"github.com/imbrooklyn/shuttle/optional"
)

// Map returns a lazy, infinite-safe Stream that applies fn exactly once to each
// consumed value while preserving encounter order. It takes O(n) time plus
// callback work, uses O(1) traversal state, and propagates downstream false.
func (s Stream[T]) Map[R any](fn func(T) R) Stream[R] {
	return Stream[R]{seq: func(yield func(R) bool) {
		s.Seq()(func(value T) bool {
			return yield(fn(value))
		})
	}}
}

// FlatMap returns a lazy Stream that traverses each fn result completely before
// requesting the next outer value. Downstream termination stops both traversals.
// It is infinite-safe for streaming, takes O(n+q) time plus callback work, and
// uses O(1) Shuttle state outside its active inner source.
func (s Stream[T]) FlatMap[R any](fn func(T) Stream[R]) Stream[R] {
	return Stream[R]{seq: func(yield func(R) bool) {
		s.Seq()(func(value T) bool {
			keepGoing := true
			fn(value).Seq()(func(inner R) bool {
				keepGoing = yield(inner)
				return keepGoing
			})
			return keepGoing
		})
	}}
}

// Filter returns a lazy, infinite-safe Stream containing values accepted by
// predicate in encounter order. It takes O(n) time plus predicate work, uses
// O(1) traversal state, and propagates downstream false.
func (s Stream[T]) Filter(predicate func(T) bool) Stream[T] {
	return Stream[T]{seq: func(yield func(T) bool) {
		s.Seq()(func(value T) bool {
			if !predicate(value) {
				return true
			}
			return yield(value)
		})
	}}
}

// FilterMap returns a lazy, infinite-safe Stream containing the values from
// present Optional results in encounter order. It takes O(n) time plus callback
// work, uses O(1) traversal state, and propagates downstream false.
func (s Stream[T]) FilterMap[R any](fn func(T) optional.Optional[R]) Stream[R] {
	return Stream[R]{seq: func(yield func(R) bool) {
		s.Seq()(func(value T) bool {
			mapped := fn(value)
			result, ok := mapped.Value()
			if !ok {
				return true
			}
			return yield(result)
		})
	}}
}

// Inspect returns a lazy, infinite-safe Stream that invokes fn before offering
// each consumed value downstream. It takes O(n) time plus callback work, uses
// O(1) traversal state, and propagates downstream false.
func (s Stream[T]) Inspect(fn func(T)) Stream[T] {
	return Stream[T]{seq: func(yield func(T) bool) {
		s.Seq()(func(value T) bool {
			fn(value)
			return yield(value)
		})
	}}
}

// Take returns a lazy Stream of at most the first n values. It panics immediately
// for negative n. Take(0) never invokes upstream, and Take(n) never probes n+1.
// It is infinite-safe, preserves prefix order, and uses O(1) state.
func (s Stream[T]) Take(n int) Stream[T] {
	if n < 0 {
		panic("stream: Take count must not be negative")
	}
	return Stream[T]{seq: func(yield func(T) bool) {
		if n == 0 {
			return
		}
		remaining := n
		s.Seq()(func(value T) bool {
			remaining--
			if !yield(value) {
				return false
			}
			return remaining != 0
		})
	}}
}

// Skip returns a lazy Stream that discards the first n values. It panics
// immediately for negative n. It is infinite-safe, preserves suffix order,
// takes O(n) time for a complete finite traversal, and uses O(1) state.
func (s Stream[T]) Skip(n int) Stream[T] {
	if n < 0 {
		panic("stream: Skip count must not be negative")
	}
	return Stream[T]{seq: func(yield func(T) bool) {
		remaining := n
		s.Seq()(func(value T) bool {
			if remaining > 0 {
				remaining--
				return true
			}
			return yield(value)
		})
	}}
}

// TakeWhile returns the longest prefix accepted by predicate. It consumes and
// tests the first rejected value, then stops upstream. It is infinite-safe,
// preserves prefix order, and uses O(1) state.
func (s Stream[T]) TakeWhile(predicate func(T) bool) Stream[T] {
	return Stream[T]{seq: func(yield func(T) bool) {
		s.Seq()(func(value T) bool {
			if !predicate(value) {
				return false
			}
			return yield(value)
		})
	}}
}

// SkipWhile returns the suffix beginning with the first value rejected by
// predicate. After that rejection it never invokes predicate again. It is
// infinite-safe, preserves suffix order, and uses O(1) state.
func (s Stream[T]) SkipWhile(predicate func(T) bool) Stream[T] {
	return Stream[T]{seq: func(yield func(T) bool) {
		skipping := true
		s.Seq()(func(value T) bool {
			if skipping {
				skipping = predicate(value)
				if skipping {
					return true
				}
			}
			return yield(value)
		})
	}}
}

// Scan returns a lazy Stream of successive accumulator values. It does not emit
// initial. Each traversal starts with a shallow copy of initial. Scan is
// infinite-safe, preserves positions, and uses O(1) state apart from the
// accumulator.
func (s Stream[T]) Scan[A any](initial A, fn func(A, T) A) Stream[A] {
	return Stream[A]{seq: func(yield func(A) bool) {
		accumulator := initial
		s.Seq()(func(value T) bool {
			accumulator = fn(accumulator, value)
			return yield(accumulator)
		})
	}}
}

// Enumerate returns a lazy Stream pairing values with zero-based consecutive
// indices. It panics rather than wrapping after math.MaxInt. Enumerate is
// infinite-safe within the int index space and uses O(1) traversal state.
func Enumerate[T any](s Stream[T]) Stream[Pair[int, T]] {
	return Stream[Pair[int, T]]{seq: func(yield func(Pair[int, T]) bool) {
		index := 0
		overflow := false
		s.Seq()(func(value T) bool {
			if overflow {
				panic("stream: Enumerate index overflow")
			}
			if !yield(Pair[int, T]{First: index, Second: value}) {
				return false
			}
			if index == math.MaxInt {
				overflow = true
			} else {
				index++
			}
			return true
		})
	}}
}

// Concat returns a lazy Stream containing the receiver followed by others in
// argument order. It snapshots the variadic descriptor slice at construction.
// Concat is infinite-safe, propagates downstream false, and uses O(1) traversal
// state in addition to the O(len(others)) construction snapshot.
func (s Stream[T]) Concat(others ...Stream[T]) Stream[T] {
	snapshot := make([]Stream[T], len(others))
	copy(snapshot, others)
	return Stream[T]{seq: func(yield func(T) bool) {
		emit := func(current Stream[T]) bool {
			keepGoing := true
			current.Seq()(func(value T) bool {
				keepGoing = yield(value)
				return keepGoing
			})
			return keepGoing
		}
		if !emit(s) {
			return
		}
		for _, current := range snapshot {
			if !emit(current) {
				return
			}
		}
	}}
}

// Zip lazily pairs values at equal positions, using left as the push-style
// driver. The right pull iterator starts only after the first left value and is
// stopped on every exit path. Zip is infinite-safe, preserves positional order,
// propagates downstream false, and uses O(1) state plus iter.Pull setup.
func Zip[A, B any](left Stream[A], right Stream[B]) Stream[Pair[A, B]] {
	return Stream[Pair[A, B]]{seq: func(yield func(Pair[A, B]) bool) {
		var next func() (B, bool)
		var stop func()
		defer func() {
			if stop != nil {
				stop()
			}
		}()

		left.Seq()(func(a A) bool {
			if next == nil {
				next, stop = iter.Pull(right.Seq())
			}
			b, ok := next()
			if !ok {
				return false
			}
			return yield(Pair[A, B]{First: a, Second: b})
		})
	}}
}
