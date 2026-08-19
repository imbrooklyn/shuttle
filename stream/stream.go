package stream

import "iter"

// Stream is a lazy, ordered sequence descriptor. Its zero value is empty.
// Copying a Stream does not copy, rewind, cache, or fork its source.
type Stream[T any] struct {
	seq iter.Seq[T]
}

// Pair holds two shallow values. Its field names, order, and field set are part
// of the public compatibility contract.
type Pair[A, B any] struct {
	// First is the left or first component.
	First A
	// Second is the right or second component.
	Second B
}

// Group holds one key and the source-ordered values assigned to that key by
// GroupBy. Its field names, order, and field set are part of the public
// compatibility contract.
type Group[K comparable, V any] struct {
	// Key is the key returned by GroupBy for the first value in this group.
	Key K
	// Values contains shallow copies in source encounter order. The slice is
	// caller-owned and stable after GroupBy returns.
	Values []V
}

// Seq returns a non-nil iterator sequence. Calling Seq does not traverse the
// source, and invoking the returned sequence inherits the source's replay
// behavior. Iteration is ordered, sequential, and propagates downstream false
// to a conforming source in O(n) time with O(1) Shuttle state.
func (s Stream[T]) Seq() iter.Seq[T] {
	if s.seq == nil {
		return emptySeq[T]
	}
	return s.seq
}

func emptySeq[T any](func(T) bool) {}
