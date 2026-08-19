package predicate

// Func is a predicate over values of type T. Its zero value is a nil function;
// constructing a composition around it is valid, and evaluation panics only if
// execution reaches the nil function call.
type Func[T any] func(T) bool

// Not returns a construction-lazy predicate that calls p exactly once and
// negates its result. Panics from p propagate unchanged.
func (p Func[T]) Not() Func[T] {
	return func(value T) bool {
		return !p(value)
	}
}

// And returns a construction-lazy predicate that evaluates p followed by
// others from left to right. Evaluation stops at the first false result. The
// variadic predicate descriptors are shallow-snapshotted during construction.
// A nil predicate skipped by short-circuiting is not called; a reached nil
// predicate panics through ordinary Go nil-function invocation.
func (p Func[T]) And(others ...Func[T]) Func[T] {
	predicates := clone(others)
	return func(value T) bool {
		if !p(value) {
			return false
		}
		for _, other := range predicates {
			if !other(value) {
				return false
			}
		}
		return true
	}
}

// Or returns a construction-lazy predicate that evaluates p followed by
// others from left to right. Evaluation stops at the first true result. The
// variadic predicate descriptors are shallow-snapshotted during construction.
// A nil predicate skipped by short-circuiting is not called; a reached nil
// predicate panics through ordinary Go nil-function invocation.
func (p Func[T]) Or(others ...Func[T]) Func[T] {
	predicates := clone(others)
	return func(value T) bool {
		if p(value) {
			return true
		}
		for _, other := range predicates {
			if other(value) {
				return true
			}
		}
		return false
	}
}

// Always returns a predicate that ignores its input and returns result.
func Always[T any](result bool) Func[T] {
	return func(T) bool {
		return result
	}
}

// Equal returns a predicate that compares each value with want using Go's ==
// operator. Interface values with dynamically non-comparable contents retain
// ordinary Go equality behavior and may panic during evaluation.
func Equal[T comparable](want T) Func[T] {
	return func(value T) bool {
		return value == want
	}
}

// EqualFunc returns a predicate that calls equal exactly once per evaluation
// with the current value first and want second. A nil equal function panics only
// when the returned predicate is evaluated.
func EqualFunc[T any](want T, equal func(T, T) bool) Func[T] {
	return func(value T) bool {
		return equal(value, want)
	}
}

// On returns a predicate that calls project exactly once and then evaluates
// predicate exactly once with the projected value. The calls are synchronous
// and ordered; their panics propagate unchanged.
func On[A, B any](project func(A) B, predicate Func[B]) Func[A] {
	return func(value A) bool {
		return predicate(project(value))
	}
}

func clone[T any](predicates []Func[T]) []Func[T] {
	if len(predicates) == 0 {
		return nil
	}
	result := make([]Func[T], len(predicates))
	copy(result, predicates)
	return result
}
