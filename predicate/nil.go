package predicate

import "reflect"

// IsNil reports whether value is a nil interface or has a nil dynamic value of
// channel, function, map, pointer, unsafe-pointer, or slice kind. It returns
// false for non-nil values and non-nilable kinds without panicking.
//
// IsNil uses reflection solely to preserve typed-nil semantics when T is an
// interface. Evaluation boxes value for reflect.ValueOf and has the associated
// reflection cost; no other predicate operation uses reflection.
func IsNil[T any](value T) bool {
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Invalid:
		return true
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice, reflect.UnsafePointer:
		return reflected.IsNil()
	default:
		return false
	}
}

// IsNotNil reports the exact logical negation of IsNil(value).
func IsNotNil[T any](value T) bool {
	return !IsNil(value)
}
