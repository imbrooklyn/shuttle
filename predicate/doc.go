// Package predicate provides small, type-safe predicate composition primitives.
//
// Func is a named function type with the underlying type func(T) bool, so its
// values can be passed directly to APIs such as optional.Optional.Filter and
// stream.Stream.Filter. Composition is lazy at construction and evaluates
// synchronously from left to right with ordinary Go short-circuit and panic
// behavior.
//
// Predicate descriptors are immutable after construction. Values captured by
// callbacks retain normal Go aliasing and concurrency semantics; callers must
// synchronize mutable captured state when predicates are evaluated concurrently.
package predicate
