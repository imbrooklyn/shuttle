package optional

// Optional contains either no value or one present value of type T. Its zero
// value is None. Presence is independent of the stored value.
type Optional[T any] struct {
	some  bool
	value T
}

// None returns an absent Optional.
func None[T any]() Optional[T] {
	return Optional[T]{}
}

// Some returns an Optional containing value, including when value is a zero or
// typed nil value.
func Some[T any](value T) Optional[T] {
	return Optional[T]{some: true, value: value}
}

// Of returns Some(value) when ok is true and None otherwise.
func Of[T any](value T, ok bool) Optional[T] {
	if !ok {
		return None[T]()
	}
	return Some(value)
}

// FromPtr returns None when ptr is nil and otherwise returns Some containing a
// shallow copy of the pointed-to value.
func FromPtr[T any](ptr *T) Optional[T] {
	if ptr == nil {
		return None[T]()
	}
	return Some(*ptr)
}

// IsSome reports whether o contains a value.
func (o Optional[T]) IsSome() bool {
	return o.some
}

// IsNone reports whether o is absent.
func (o Optional[T]) IsNone() bool {
	return !o.some
}

// Value returns the stored value and true when o is present. For None it
// returns the zero value of T and false.
func (o Optional[T]) Value() (T, bool) {
	if !o.some {
		var zero T
		return zero, false
	}
	return o.value, true
}

// OrZero returns the stored value when present and the zero value of T
// otherwise.
func (o Optional[T]) OrZero() T {
	if !o.some {
		var zero T
		return zero
	}
	return o.value
}

// OrElse returns the stored value when present and fallback otherwise.
func (o Optional[T]) OrElse(fallback T) T {
	if o.some {
		return o.value
	}
	return fallback
}

// OrElseGet returns the stored value when present. Otherwise it invokes
// fallback exactly once and returns its result.
func (o Optional[T]) OrElseGet(fallback func() T) T {
	if o.some {
		return o.value
	}
	return fallback()
}

// Must returns the stored value. It panics when o is None.
func (o Optional[T]) Must() T {
	if !o.some {
		panic("optional: value is absent")
	}
	return o.value
}

// Ptr returns nil for None. For Some it returns a pointer to a new shallow copy
// of the stored value.
func (o Optional[T]) Ptr() *T {
	if !o.some {
		return nil
	}
	value := o.value
	return &value
}

// Map eagerly applies fn to a present value and returns Some of the result. It
// returns None without invoking fn when o is absent.
func (o Optional[T]) Map[R any](fn func(T) R) Optional[R] {
	if !o.some {
		return None[R]()
	}
	return Some(fn(o.value))
}

// FlatMap eagerly applies fn to a present value and returns its Optional result.
// It returns None without invoking fn when o is absent.
func (o Optional[T]) FlatMap[R any](fn func(T) Optional[R]) Optional[R] {
	if !o.some {
		return None[R]()
	}
	return fn(o.value)
}

// Filter returns o when it is present and predicate accepts its value. It
// returns None for an absent or rejected value.
func (o Optional[T]) Filter(predicate func(T) bool) Optional[T] {
	if !o.some || !predicate(o.value) {
		return None[T]()
	}
	return o
}

// Inspect invokes fn once for a present value and returns o unchanged.
func (o Optional[T]) Inspect(fn func(T)) Optional[T] {
	if o.some {
		fn(o.value)
	}
	return o
}

// Match invokes and returns exactly one branch: onSome for a present value or
// onNone for an absent value.
func (o Optional[T]) Match[R any](onSome func(T) R, onNone func() R) R {
	if o.some {
		return onSome(o.value)
	}
	return onNone()
}

// ZipWith returns None unless both Optionals are present. When both are
// present, it invokes combine once and returns Some of the result.
func (o Optional[T]) ZipWith[U, R any](other Optional[U], combine func(T, U) R) Optional[R] {
	if !o.some || !other.some {
		return None[R]()
	}
	return Some(combine(o.value, other.value))
}

// Or returns o when it is present and other otherwise.
func (o Optional[T]) Or(other Optional[T]) Optional[T] {
	if o.some {
		return o
	}
	return other
}

// OrGet returns o when it is present. Otherwise it invokes other exactly once
// and returns its result.
func (o Optional[T]) OrGet(other func() Optional[T]) Optional[T] {
	if o.some {
		return o
	}
	return other()
}

// Flatten returns the inner Optional when nested is present and None otherwise.
func Flatten[T any](nested Optional[Optional[T]]) Optional[T] {
	if !nested.some {
		return None[T]()
	}
	return nested.value
}

// Equal reports state-and-value equality for Optionals with comparable element
// types.
func Equal[T comparable](a, b Optional[T]) bool {
	if a.some != b.some {
		return false
	}
	if !a.some {
		return true
	}
	return a.value == b.value
}

// EqualFunc reports state-and-value equality using equal for two present
// values. It does not invoke equal when either Optional is absent.
func EqualFunc[T any](a, b Optional[T], equal func(T, T) bool) bool {
	if a.some != b.some {
		return false
	}
	if !a.some {
		return true
	}
	return equal(a.value, b.value)
}

// IsZero reports whether o is None.
func (o Optional[T]) IsZero() bool {
	return !o.some
}
