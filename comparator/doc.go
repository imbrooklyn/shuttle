// Package comparator provides small, type-safe primitives for constructing and
// composing three-way comparison functions.
//
// A Func value can be passed directly to standard-library sorting functions and
// to Shuttle Stream comparison operations. Ordered and custom projected levels
// can be composed fluently with independent directions and key types.
// Comparators are construction-lazy, synchronous, sequential, and free of
// hidden caching or concurrency.
package comparator
