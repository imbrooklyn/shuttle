package stream

import (
	"cmp"
	"math"

	"github.com/imbrooklyn/shuttle/optional"
)

// Collect consumes s into a new slice in encounter order. It returns nil for an
// empty Stream. It takes O(n) time and returned memory and requires finite input.
func (s Stream[T]) Collect() []T {
	var result []T
	s.Seq()(func(value T) bool {
		result = append(result, value)
		return true
	})
	return result
}

// AppendTo appends all values to dst in encounter order and preserves its named
// slice type. Empty input returns dst unchanged, including nilness. It takes
// O(n) time, uses ordinary append capacity growth, and requires finite input.
func (s Stream[T]) AppendTo[S ~[]T](dst S) S {
	result := dst
	s.Seq()(func(value T) bool {
		result = append(result, value)
		return true
	})
	return result
}

// Count consumes s and returns its element count. It panics instead of wrapping
// if another element would overflow int. Count uses O(1) state and requires
// finite input to return.
func (s Stream[T]) Count() int {
	count := 0
	s.Seq()(func(T) bool {
		if count == math.MaxInt {
			panic("stream: Count overflow")
		}
		count++
		return true
	})
	return count
}

// First returns the first value and stops upstream, or None when s is empty. It
// is infinite-safe and uses O(1) state.
func (s Stream[T]) First() optional.Optional[T] {
	result := optional.None[T]()
	s.Seq()(func(value T) bool {
		result = optional.Some(value)
		return false
	})
	return result
}

// Last consumes s and returns its last value, or None when s is empty. It uses
// O(1) state and requires finite input.
func (s Stream[T]) Last() optional.Optional[T] {
	result := optional.None[T]()
	s.Seq()(func(value T) bool {
		result = optional.Some(value)
		return true
	})
	return result
}

// Find returns the first value accepted by predicate and stops upstream, or
// None after exhaustion without a match. It uses O(1) state and is conditionally
// safe for infinite input because only a match guarantees termination.
func (s Stream[T]) Find(predicate func(T) bool) optional.Optional[T] {
	result := optional.None[T]()
	s.Seq()(func(value T) bool {
		if !predicate(value) {
			return true
		}
		result = optional.Some(value)
		return false
	})
	return result
}

// At returns the zero-based element at index and stops upstream. A negative
// index returns None without invoking upstream. It is infinite-safe for every
// finite non-negative index and uses O(1) state.
func (s Stream[T]) At(index int) optional.Optional[T] {
	if index < 0 {
		return optional.None[T]()
	}
	current := 0
	result := optional.None[T]()
	s.Seq()(func(value T) bool {
		if current != index {
			current++
			return true
		}
		result = optional.Some(value)
		return false
	})
	return result
}

// Any reports whether predicate accepts any value and stops at the first match.
// It uses O(1) state and is conditionally safe for infinite input.
func (s Stream[T]) Any(predicate func(T) bool) bool {
	found := false
	s.Seq()(func(value T) bool {
		found = predicate(value)
		return !found
	})
	return found
}

// All reports whether predicate accepts every value and stops at the first
// rejection. It returns true for empty input. It uses O(1) state and is
// conditionally safe for infinite input.
func (s Stream[T]) All(predicate func(T) bool) bool {
	all := true
	s.Seq()(func(value T) bool {
		all = predicate(value)
		return all
	})
	return all
}

// None reports whether predicate accepts no value and stops at the first match.
// It returns true for empty input. It uses O(1) state and is conditionally safe
// for infinite input.
func (s Stream[T]) None(predicate func(T) bool) bool {
	return !s.Any(predicate)
}

// Contains reports whether s contains value by == and stops at the first match.
// It uses O(1) state and is conditionally safe for infinite input.
func Contains[T comparable](s Stream[T], value T) bool {
	return s.Any(func(current T) bool { return current == value })
}

// ForEach invokes fn once for every value in encounter order. It uses O(1)
// state and requires finite input to return normally.
func (s Stream[T]) ForEach(fn func(T)) {
	s.Seq()(func(value T) bool {
		fn(value)
		return true
	})
}

// ForEachErr invokes fn in encounter order and stops at the first non-nil error,
// returning that error unchanged. It uses O(1) state and is conditionally safe
// for infinite input because an error can terminate traversal.
func (s Stream[T]) ForEachErr(fn func(T) error) error {
	var result error
	s.Seq()(func(value T) bool {
		result = fn(value)
		return result == nil
	})
	return result
}

// Reduce performs a left fold beginning with a shallow copy of initial. Empty
// input returns initial without invoking fn. Reduce uses O(1) accumulator state
// and requires finite input.
func (s Stream[T]) Reduce[A any](initial A, fn func(A, T) A) A {
	result := initial
	s.Seq()(func(value T) bool {
		result = fn(result, value)
		return true
	})
	return result
}

// ReduceFirst performs a left fold beginning with the first value. It returns
// None for empty input and does not invoke fn for a singleton. ReduceFirst uses
// O(1) state and requires finite input.
func (s Stream[T]) ReduceFirst(fn func(T, T) T) optional.Optional[T] {
	var result T
	present := false
	s.Seq()(func(value T) bool {
		if !present {
			result = value
			present = true
		} else {
			result = fn(result, value)
		}
		return true
	})
	return optional.Of(result, present)
}

// MinFunc consumes s and returns its least value according to compare, retaining
// the first value on ties. It returns None for empty input. It performs O(n)
// comparisons, uses O(1) state, and requires finite input.
func (s Stream[T]) MinFunc(compare func(T, T) int) optional.Optional[T] {
	var candidate T
	present := false
	s.Seq()(func(value T) bool {
		if !present {
			candidate = value
			present = true
		} else if compare(value, candidate) < 0 {
			candidate = value
		}
		return true
	})
	return optional.Of(candidate, present)
}

// MaxFunc consumes s and returns its greatest value according to compare,
// retaining the first value on ties. It returns None for empty input. It
// performs O(n) comparisons, uses O(1) state, and requires finite input.
func (s Stream[T]) MaxFunc(compare func(T, T) int) optional.Optional[T] {
	var candidate T
	present := false
	s.Seq()(func(value T) bool {
		if !present {
			candidate = value
			present = true
		} else if compare(value, candidate) > 0 {
			candidate = value
		}
		return true
	})
	return optional.Of(candidate, present)
}

// MinBy consumes s and returns the element with the least ordered key, invoking
// key exactly once per element and retaining the first element on ties. It uses
// O(1) state, takes O(n) time plus key work, and requires finite input.
func (s Stream[T]) MinBy[K cmp.Ordered](key func(T) K) optional.Optional[T] {
	var candidate T
	var candidateKey K
	present := false
	s.Seq()(func(value T) bool {
		currentKey := key(value)
		if !present || cmp.Compare(currentKey, candidateKey) < 0 {
			candidate = value
			candidateKey = currentKey
			present = true
		}
		return true
	})
	return optional.Of(candidate, present)
}

// MaxBy consumes s and returns the element with the greatest ordered key,
// invoking key exactly once per element and retaining the first element on ties.
// It uses O(1) state, takes O(n) time plus key work, and requires finite input.
func (s Stream[T]) MaxBy[K cmp.Ordered](key func(T) K) optional.Optional[T] {
	var candidate T
	var candidateKey K
	present := false
	s.Seq()(func(value T) bool {
		currentKey := key(value)
		if !present || cmp.Compare(currentKey, candidateKey) > 0 {
			candidate = value
			candidateKey = currentKey
			present = true
		}
		return true
	})
	return optional.Of(candidate, present)
}

// Min consumes s and returns its least naturally ordered value, retaining the
// first value on ties. It returns None for empty input. It takes O(n) time, uses
// O(1) state, and requires finite input.
func Min[T cmp.Ordered](s Stream[T]) optional.Optional[T] {
	return s.MinFunc(cmp.Compare[T])
}

// Max consumes s and returns its greatest naturally ordered value, retaining
// the first value on ties. It returns None for empty input. It takes O(n) time,
// uses O(1) state, and requires finite input.
func Max[T cmp.Ordered](s Stream[T]) optional.Optional[T] {
	return s.MaxFunc(cmp.Compare[T])
}

// ToMap consumes s into a new non-nil map. It invokes key then value once per
// element, and later duplicate keys replace earlier values. It returns a non-nil
// map, uses expected O(u) result memory and O(n) time, and requires finite input.
func (s Stream[T]) ToMap[K comparable, V any](key func(T) K, value func(T) V) map[K]V {
	result := make(map[K]V)
	s.Seq()(func(element T) bool {
		currentKey := key(element)
		currentValue := value(element)
		result[currentKey] = currentValue
		return true
	})
	return result
}

// ToMapWith consumes s into a new non-nil map. For duplicate keys it invokes
// merge(existing, incoming); first occurrences do not invoke merge. It returns
// a non-nil map, uses expected O(u) result memory and O(n) time plus merge work,
// and requires finite input.
func (s Stream[T]) ToMapWith[K comparable, V any](key func(T) K, value func(T) V, merge func(V, V) V) map[K]V {
	result := make(map[K]V)
	s.Seq()(func(element T) bool {
		currentKey := key(element)
		incoming := value(element)
		existing, ok := result[currentKey]
		if ok {
			incoming = merge(existing, incoming)
		}
		result[currentKey] = incoming
		return true
	})
	return result
}

// GroupBy consumes s into caller-owned groups ordered by each key's first
// encounter. Values within a group preserve source order. Empty input returns
// nil. It uses expected O(n+g) result and indexing memory, expected O(n) time,
// and requires finite input.
func (s Stream[T]) GroupBy[K comparable](key func(T) K) []Group[K, T] {
	var groups []Group[K, T]
	indices := make(map[K]int)
	s.Seq()(func(value T) bool {
		currentKey := key(value)
		index, ok := indices[currentKey]
		if !ok {
			index = len(groups)
			indices[currentKey] = index
			groups = append(groups, Group[K, T]{Key: currentKey})
		}
		groups[index].Values = append(groups[index].Values, value)
		return true
	})
	return groups
}
