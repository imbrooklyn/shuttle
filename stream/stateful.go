package stream

import (
	"cmp"
	"slices"
)

// DistinctBy returns a lazy Stream containing the first value for each key in
// encounter order. Its per-traversal key set uses O(u) additional memory and
// expected O(n) time. It propagates downstream false and is conditionally safe
// for infinite input because its key set or a duplicate-only suffix may be
// unbounded.
func (s Stream[T]) DistinctBy[K comparable](key func(T) K) Stream[T] {
	return Stream[T]{seq: func(yield func(T) bool) {
		seen := make(map[K]struct{})
		s.Seq()(func(value T) bool {
			currentKey := key(value)
			if _, ok := seen[currentKey]; ok {
				return true
			}
			seen[currentKey] = struct{}{}
			return yield(value)
		})
	}}
}

// Distinct returns the first occurrence of each comparable value in encounter
// order. Its state is independent for every traversal. Expected time is O(n),
// memory is O(u), downstream false is propagated, and infinite-input safety is
// conditional.
func Distinct[T comparable](s Stream[T]) Stream[T] {
	return s.DistinctBy(func(value T) T { return value })
}

// Chunk lazily partitions s into independently owned slices of at most size.
// It emits a final non-empty partial chunk and panics immediately for size <= 0.
// Chunk is infinite-safe, preserves order, uses O(size) working memory, and
// propagates downstream false without consuming a later chunk.
func Chunk[T any](s Stream[T], size int) Stream[[]T] {
	if size <= 0 {
		panic("stream: Chunk size must be positive")
	}
	return Stream[[]T]{seq: func(yield func([]T) bool) {
		buffer := make([]T, 0, size)
		keepGoing := true
		s.Seq()(func(value T) bool {
			buffer = append(buffer, value)
			if len(buffer) < size {
				return true
			}
			chunk := make([]T, len(buffer))
			copy(chunk, buffer)
			keepGoing = yield(chunk)
			buffer = buffer[:0]
			return keepGoing
		})
		if keepGoing && len(buffer) != 0 {
			chunk := make([]T, len(buffer))
			copy(chunk, buffer)
			yield(chunk)
		}
	}}
}

// Window returns the full, independently owned sliding windows of size from s.
// It is equivalent to WindowStep(s, size, 1) and panics for size <= 0. Window is
// infinite-safe, ordered, uses O(size) working memory, and propagates downstream
// false before consuming a later window.
func Window[T any](s Stream[T], size int) Stream[[]T] {
	if size <= 0 {
		panic("stream: Window size must be positive")
	}
	return WindowStep(s, size, 1)
}

// WindowStep returns independently owned full windows at starts 0, step,
// 2*step, and so on. It supports overlap and gaps and panics immediately unless
// size and step are positive. WindowStep is infinite-safe, ordered, uses O(size)
// working memory, takes O(n+w*size) time including owned copies, and propagates
// downstream false before consuming gaps or later windows.
func WindowStep[T any](s Stream[T], size, step int) Stream[[]T] {
	if size <= 0 {
		panic("stream: WindowStep size must be positive")
	}
	if step <= 0 {
		panic("stream: WindowStep step must be positive")
	}
	return Stream[[]T]{seq: func(yield func([]T) bool) {
		buffer := make([]T, 0, size)
		gap := 0
		s.Seq()(func(value T) bool {
			if gap != 0 {
				gap--
				return true
			}
			buffer = append(buffer, value)
			if len(buffer) < size {
				return true
			}

			window := make([]T, size)
			copy(window, buffer)
			if !yield(window) {
				return false
			}

			if step < size {
				copy(buffer, buffer[step:])
				buffer = buffer[:size-step]
			} else {
				buffer = buffer[:0]
				gap = step - size
			}
			return true
		})
	}}
}

// SortedFunc returns a construction-lazy finite-only barrier that recollects
// and stably sorts s on every traversal without modifying source-owned storage.
// It takes O(n log n) comparisons, retains O(n) values, propagates downstream
// false only after the barrier, and cannot produce output for infinite input.
func (s Stream[T]) SortedFunc(compare func(T, T) int) Stream[T] {
	return Stream[T]{seq: func(yield func(T) bool) {
		values := s.Collect()
		slices.SortStableFunc(values, compare)
		for _, value := range values {
			if !yield(value) {
				return
			}
		}
	}}
}

// Sorted returns a construction-lazy stable natural-order sort of s. It is a
// finite-only barrier and recollects the source on every traversal. It takes
// O(n log n) comparisons, retains O(n) values, and applies downstream false only
// after the barrier.
func Sorted[T cmp.Ordered](s Stream[T]) Stream[T] {
	return s.SortedFunc(cmp.Compare[T])
}

// Reverse returns a construction-lazy finite-only barrier that recollects s on
// every traversal and emits exact reverse encounter order. It takes O(n) time
// and memory, applies downstream false only after the barrier, and cannot
// produce output for infinite input.
func (s Stream[T]) Reverse() Stream[T] {
	return Stream[T]{seq: func(yield func(T) bool) {
		values := s.Collect()
		for index := len(values) - 1; index >= 0; index-- {
			if !yield(values[index]) {
				return
			}
		}
	}}
}
