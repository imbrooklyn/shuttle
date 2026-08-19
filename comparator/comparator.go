package comparator

import "cmp"

// Func compares left and right. A negative result orders left before right,
// zero makes them equivalent at that ordering level, and a positive result
// orders left after right. Only the sign of the result is meaningful. The zero
// value is a nil function and panics through ordinary nil-function invocation
// only when evaluation reaches it.
type Func[T any] func(left, right T) int

// Ordered returns the natural ascending comparator for T. Integer, string,
// floating-point, and NaN behavior is exactly that of cmp.Compare.
func Ordered[T cmp.Ordered]() Func[T] {
	return cmp.Compare[T]
}

// By returns a construction-lazy ascending comparator over an ordered key.
// Each evaluation calls key with left exactly once, then with right exactly
// once, and compares those keys using cmp.Compare. It does not cache keys
// between evaluations.
func By[T any, K cmp.Ordered](key func(T) K) Func[T] {
	return func(left, right T) int {
		leftKey := key(left)
		rightKey := key(right)
		return cmp.Compare(leftKey, rightKey)
	}
}

// ByDescending returns a construction-lazy descending comparator over an
// ordered key. Each evaluation calls key with left exactly once, then with
// right exactly once, and compares the right key with the left key using
// cmp.Compare. It does not cache keys between evaluations.
func ByDescending[T any, K cmp.Ordered](key func(T) K) Func[T] {
	return func(left, right T) int {
		leftKey := key(left)
		rightKey := key(right)
		return cmp.Compare(rightKey, leftKey)
	}
}

// On returns a construction-lazy comparator that projects both operands and
// delegates to compare. Each evaluation calls project with left exactly once,
// then with right exactly once, and finally calls compare with the projected
// values in left-right order exactly once. It does not cache projections
// between evaluations. The unnamed compare parameter also accepts Func[B] and
// other compatible named function types without conversion.
func On[A, B any](project func(A) B, compare func(B, B) int) Func[A] {
	return func(left, right A) int {
		leftProjected := project(left)
		rightProjected := project(right)
		return compare(leftProjected, rightProjected)
	}
}

// OnDescending returns a construction-lazy comparator that projects both
// operands and reverses the ordering represented by compare. Each evaluation
// calls project with left exactly once, then with right exactly once, and
// finally calls compare with the projected values in left-right order exactly
// once. It reverses only the result sign, so math.MinInt is handled without
// overflow, and does not cache projections between evaluations.
func OnDescending[A, B any](project func(A) B, compare func(B, B) int) Func[A] {
	return func(left, right A) int {
		leftProjected := project(left)
		rightProjected := project(right)
		return reverseResult(compare(leftProjected, rightProjected))
	}
}

// Reverse returns a construction-lazy comparator that reverses the complete
// ordering represented by c. Each evaluation calls c with left and right
// exactly once, then reverses only its result sign. The returned nonzero result
// is normalized to -1 or 1, so math.MinInt is handled without overflow. Panics
// from c propagate unchanged.
func (c Func[T]) Reverse() Func[T] {
	return func(left, right T) int {
		return reverseResult(c(left, right))
	}
}

// Then returns a construction-lazy lexicographic composition. It evaluates c
// first, followed by others from left to right, and stops at the first nonzero
// result. The variadic comparator descriptors are shallow-snapshotted during
// construction. A skipped nil comparator is not called; a reached nil
// comparator panics through ordinary Go nil-function invocation.
func (c Func[T]) Then(others ...Func[T]) Func[T] {
	comparators := clone(others)
	return func(left, right T) int {
		if result := c(left, right); result != 0 {
			return result
		}
		for _, other := range comparators {
			if result := other(left, right); result != 0 {
				return result
			}
		}
		return 0
	}
}

// ThenBy returns a construction-lazy lexicographic composition with one
// ascending ordered-key level appended. Each evaluation calls c first and
// returns immediately when its result is nonzero. Only after a zero result does
// it call key with left exactly once, then with right exactly once, and compare
// those keys using cmp.Compare.
func (c Func[T]) ThenBy[K cmp.Ordered](key func(T) K) Func[T] {
	return func(left, right T) int {
		if result := c(left, right); result != 0 {
			return result
		}
		leftKey := key(left)
		rightKey := key(right)
		return cmp.Compare(leftKey, rightKey)
	}
}

// ThenByDescending returns a construction-lazy lexicographic composition with
// one descending ordered-key level appended. Each evaluation calls c first and
// returns immediately when its result is nonzero. Only after a zero result does
// it call key with left exactly once, then with right exactly once, and compare
// the right key with the left key using cmp.Compare.
func (c Func[T]) ThenByDescending[K cmp.Ordered](key func(T) K) Func[T] {
	return func(left, right T) int {
		if result := c(left, right); result != 0 {
			return result
		}
		leftKey := key(left)
		rightKey := key(right)
		return cmp.Compare(rightKey, leftKey)
	}
}

// ThenOn returns a construction-lazy lexicographic composition with one custom
// projected ordering level appended. Each evaluation calls c first and returns
// immediately when its result is nonzero. Only after a zero result does it
// project left exactly once, project right exactly once, and call compare with
// those projected values in left-right order exactly once. The unnamed compare
// parameter also accepts Func[K] and other compatible named function types
// without conversion.
func (c Func[T]) ThenOn[K any](project func(T) K, compare func(K, K) int) Func[T] {
	return func(left, right T) int {
		if result := c(left, right); result != 0 {
			return result
		}
		leftProjected := project(left)
		rightProjected := project(right)
		return compare(leftProjected, rightProjected)
	}
}

// ThenOnDescending returns a construction-lazy lexicographic composition with
// one reversed custom projected ordering level appended. Each evaluation calls
// c first and returns immediately when its result is nonzero. Only after a zero
// result does it project left exactly once, project right exactly once, call
// compare in left-right order exactly once, and safely reverse the result sign.
func (c Func[T]) ThenOnDescending[K any](project func(T) K, compare func(K, K) int) Func[T] {
	return func(left, right T) int {
		if result := c(left, right); result != 0 {
			return result
		}
		leftProjected := project(left)
		rightProjected := project(right)
		return reverseResult(compare(leftProjected, rightProjected))
	}
}

func clone[T any](comparators []Func[T]) []Func[T] {
	if len(comparators) == 0 {
		return nil
	}
	result := make([]Func[T], len(comparators))
	copy(result, comparators)
	return result
}

func reverseResult(result int) int {
	switch {
	case result < 0:
		return 1
	case result > 0:
		return -1
	default:
		return 0
	}
}
