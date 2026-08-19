# Shuttle v1 Public API Specification

<!-- markdownlint-disable MD013 MD024 -->

Status: normative implementation specification  
Module: `github.com/imbrooklyn/shuttle`  
Minimum language version: Go 1.27  
RC validation baseline: `go1.27rc3`

This document defines the complete Shuttle v1 public API and its observable behavior. The key words **must**, **must not**, **should**, and **may** are normative. Public identifiers not listed here must not be added to v1 without an API review and a specification change.

## 1. Specification conventions

For complexity statements:

- `n` is the number of upstream elements consumed;
- `m` is the number of elements available from a second input when an operation has two inputs;
- `q` is the total number of inner elements consumed by `FlatMap`;
- `k` is a requested count, chunk size, or window size as applicable;
- `u` is the number of distinct keys encountered;
- `g` is the number of groups;
- `w` is the number of emitted windows; and
- expected `O(1)` map access assumes a well-behaved hash function.

“Additional memory” excludes storage already owned by the source. For a terminal it includes the returned collection. For a transformation it includes traversal working state; separately owned emitted slices are stated explicitly.

Infinite-input classifications are:

- **IS — infinite-safe:** incremental with bounded operator state;
- **C — conditional:** can short-circuit or stream useful output, but termination or bounded memory depends on data; and
- **F — finite-only:** requires upstream exhaustion to emit its first value or return.

All copies are shallow Go assignments unless this specification explicitly requires a new backing array.

## 2. Common behavioral contract

### 2.1 Callback execution

Callbacks and comparators execute synchronously and sequentially within one comparator or predicate evaluation, Optional operation, or Stream traversal. Shuttle must not invoke a user callback concurrently with itself and must not start worker goroutines. Callback side effects occur in the order documented for the operation.

A Stream source must call `yield` serially and wait for each call to return. Concurrent calls to one traversal's `yield` are unsupported and may race or panic. Shuttle does not guarantee callback goroutine identity for an externally supplied source; it guarantees only that Shuttle introduces no callback concurrency or worker goroutine.

A nil callback is not rejected merely because an operator is constructed. It panics through ordinary Go nil-function invocation only if execution reaches that callback. An empty or absent branch that does not need the callback must not invoke it.

Shuttle must not recover a panic from a callback, comparator, source, generator, downstream yield function, JSON implementation of `T`, or the Go runtime.

### 2.2 Invalid numeric arguments

The following invalid arguments must panic immediately when the function or method is called, before any stream traversal:

| API | Invalid argument |
| --- | --- |
| `RangeStep` | `step == 0` |
| `RepeatN` | `n < 0` |
| `Take`, `Skip` | `n < 0` |
| `Chunk` | `size <= 0` |
| `Window` | `size <= 0` |
| `WindowStep` | `size <= 0` or `step <= 0` |

The panic value or text is not specified. Tests must assert that a panic occurs, not match its text.

### 2.3 Map-key failures

Go permits an interface type to satisfy `comparable` even though a particular dynamic value stored in it may not be comparable. Any Shuttle operation that inserts or looks up such a value in a map has ordinary Go map behavior and may panic. This applies to `Distinct`, `DistinctBy`, `ToMap`, `ToMapWith`, and `GroupBy`.

### 2.4 Immutability and thread safety

Public value methods do not mutate an `Optional` or `Stream`, and comparator or predicate composition does not mutate an existing `Func`. That fact alone does not make referenced values, sources, callbacks, or destinations safe for concurrent access. Concurrent comparator or predicate evaluation and concurrent Stream traversal are supported only to the extent that every reached callback, captured value, source, and destination is safe for concurrent use. Shuttle adds no cross-evaluation or cross-traversal lock.

Nil pointers to these value types are not a third logical state. Except for the explicit nil-receiver rule of `(*Optional[T]).UnmarshalJSON`, invoking a value-receiver method through a nil `*Optional[T]` or `*Stream[T]` has ordinary Go nil-pointer dereference behavior.

## 3. Package `optional`

Import path:

```go
import "github.com/imbrooklyn/shuttle/optional"
```

### 3.1 Complete public declaration inventory

The package must export exactly the following v1 API, apart from methods automatically associated with standard interfaces:

```go
package optional

type Optional[T any] struct {
    // unexported
}

func None[T any]() Optional[T]
func Some[T any](value T) Optional[T]
func Of[T any](value T, ok bool) Optional[T]
func FromPtr[T any](ptr *T) Optional[T]

func (o Optional[T]) IsSome() bool
func (o Optional[T]) IsNone() bool
func (o Optional[T]) Value() (T, bool)
func (o Optional[T]) OrZero() T
func (o Optional[T]) OrElse(fallback T) T
func (o Optional[T]) OrElseGet(fallback func() T) T
func (o Optional[T]) Must() T
func (o Optional[T]) Ptr() *T

func (o Optional[T]) Map[R any](fn func(T) R) Optional[R]
func (o Optional[T]) FlatMap[R any](fn func(T) Optional[R]) Optional[R]
func (o Optional[T]) Filter(predicate func(T) bool) Optional[T]
func (o Optional[T]) Inspect(fn func(T)) Optional[T]
func (o Optional[T]) Match[R any](onSome func(T) R, onNone func() R) R
func (o Optional[T]) ZipWith[U, R any](other Optional[U], combine func(T, U) R) Optional[R]
func (o Optional[T]) Or(other Optional[T]) Optional[T]
func (o Optional[T]) OrGet(other func() Optional[T]) Optional[T]

func Flatten[T any](nested Optional[Optional[T]]) Optional[T]
func Equal[T comparable](a, b Optional[T]) bool
func EqualFunc[T any](a, b Optional[T], equal func(T, T) bool) bool

func (o Optional[T]) IsZero() bool
func (o Optional[T]) MarshalJSON() ([]byte, error)
func (o *Optional[T]) UnmarshalJSON(data []byte) error
```

There are no exported sentinel errors, state enums, constructors named `New`, extraction aliases, or formatting methods in v1.

### 3.2 Type and state model

```go
type Optional[T any] struct { /* unexported */ }
```

An Optional has one of two states:

| State | `IsSome` | `IsNone` | `Value` result |
| --- | ---: | ---: | --- |
| None | false | true | zero `T`, false |
| Some(value) | true | false | `value`, true |

The zero value of `Optional[T]` must be None. There is no invalid state.

Presence is independent of the value. `Some(0)`, `Some("")`, and `Some(false)` are present. For nil-capable types, a typed nil is also present:

```go
var slice []int
var ptr *int

a := optional.Some(slice)          // Some([]int(nil))
b := optional.Some(ptr)            // Some((*int)(nil))
c := optional.Some[any](nil)       // Some(nil interface value)
```

`optional.Some(nil)` without a type argument or typed operand does not compile because `T` cannot be inferred. This must not be worked around with an `any`-accepting constructor.

When `T` is comparable, `Optional[T]` must be comparable and direct `==` must follow the state model: None equals None, Some equals Some when their values compare equal, and None never equals Some. Comparison has normal Go behavior for interface values with dynamically non-comparable contents.

### 3.3 Constructors

#### `None`

```go
func None[T any]() Optional[T]
```

Returns an absent Optional. It is equal in state and behavior to the zero value. Time and memory are `O(1)` with no allocation.

#### `Some`

```go
func Some[T any](value T) Optional[T]
```

Returns a present Optional containing a shallow copy of `value`, even when `value` is the zero value or a typed nil. Time and memory are `O(1)` apart from copying `T`; the constructor itself must not intentionally allocate.

#### `Of`

```go
func Of[T any](value T, ok bool) Optional[T]
```

Returns `Some(value)` when `ok` is true and `None[T]()` when `ok` is false. When false, the supplied value is ignored. This constructor is intended to adapt Go's `(value, ok)` convention.

#### `FromPtr`

```go
func FromPtr[T any](ptr *T) Optional[T]
```

Returns None when `ptr == nil`; otherwise returns Some with a shallow copy of `*ptr`. Later assigning through `ptr` does not replace the stored value, although reference-like fields inside `T` retain ordinary shallow aliases.

### 3.4 Inspection and extraction

#### `IsSome` and `IsNone`

```go
func (o Optional[T]) IsSome() bool
func (o Optional[T]) IsNone() bool
```

Report presence and absence respectively. They are exact complements.

#### `Value`

```go
func (o Optional[T]) Value() (T, bool)
```

Returns the stored value and true for Some. It returns the zero value of `T` and false for None. It never panics because of absence.

#### `OrZero`

```go
func (o Optional[T]) OrZero() T
```

Returns the stored value for Some and the zero value of `T` for None.

#### `OrElse`

```go
func (o Optional[T]) OrElse(fallback T) T
```

Returns the stored value for Some and `fallback` for None. Go evaluates the fallback argument before the call; use `OrElseGet` when evaluation must be conditional.

#### `OrElseGet`

```go
func (o Optional[T]) OrElseGet(fallback func() T) T
```

Returns the stored value without invoking `fallback` for Some. For None, invokes `fallback` exactly once and returns its result.

#### `Must`

```go
func (o Optional[T]) Must() T
```

Returns the stored value for Some. It panics for None. `Must` is intended for invariants and tests, not ordinary absence control flow.

#### `Ptr`

```go
func (o Optional[T]) Ptr() *T
```

Returns nil for None. For Some, returns a pointer to a new shallow copy of the stored value. Every simultaneously live result from separate calls is independent: assigning through one pointer must not change the Optional or another pointer returned by `Ptr`. The Some path normally allocates because the copy escapes.

### 3.5 Transformation and composition

Every operation in this section is eager. Its documented callback runs during the method call, not during later extraction.

#### `Map`

```go
func (o Optional[T]) Map[R any](fn func(T) R) Optional[R]
```

For None, returns `None[R]()` without invoking `fn`. For Some, invokes `fn` exactly once with the stored value and returns Some of the result, including a zero or typed nil result.

#### `FlatMap`

```go
func (o Optional[T]) FlatMap[R any](fn func(T) Optional[R]) Optional[R]
```

For None, returns `None[R]()` without invoking `fn`. For Some, invokes `fn` exactly once and returns its Optional unchanged in logical state and value. No additional flattening or nil interpretation occurs.

#### `Filter`

```go
func (o Optional[T]) Filter(predicate func(T) bool) Optional[T]
```

For None, returns None without invoking `predicate`. For Some, invokes `predicate` exactly once. It returns the original Optional when true and None when false.

#### `Inspect`

```go
func (o Optional[T]) Inspect(fn func(T)) Optional[T]
```

For None, returns None without invoking `fn`. For Some, invokes `fn` exactly once and then returns the original Optional. `Inspect` is for observation; callback mutations through reference-like values have normal Go aliasing effects.

#### `Match`

```go
func (o Optional[T]) Match[R any](onSome func(T) R, onNone func() R) R
```

For Some, invokes `onSome` exactly once and does not invoke `onNone`. For None, invokes `onNone` exactly once and does not invoke `onSome`. It returns the selected callback's result.

#### `ZipWith`

```go
func (o Optional[T]) ZipWith[U, R any](other Optional[U], combine func(T, U) R) Optional[R]
```

If either Optional is None, returns `None[R]()` without invoking `combine`. If both are Some, invokes `combine` exactly once with receiver value first and `other` value second, then returns Some of the result. Evaluation of the `other` expression itself follows normal eager Go argument evaluation.

#### `Or`

```go
func (o Optional[T]) Or(other Optional[T]) Optional[T]
```

Returns the receiver when it is Some; otherwise returns `other`. Evaluation of the `other` expression is eager. The chosen Optional is returned by value with shallow semantics.

#### `OrGet`

```go
func (o Optional[T]) OrGet(other func() Optional[T]) Optional[T]
```

Returns the receiver without invoking `other` when it is Some. For None, invokes `other` exactly once and returns its result.

#### `Flatten`

```go
func Flatten[T any](nested Optional[Optional[T]]) Optional[T]
```

Returns None when the outer Optional is None. When the outer Optional is Some, returns the inner Optional. This is a package function because it is defined only for the nested receiver shape.

#### `Equal`

```go
func Equal[T comparable](a, b Optional[T]) bool
```

Returns true for two None values, false for a presence mismatch, and `aValue == bValue` for two Some values. It is equivalent to direct `==` for valid comparable values and provides an explicit generic operation.

#### `EqualFunc`

```go
func EqualFunc[T any](a, b Optional[T], equal func(T, T) bool) bool
```

Returns true for two None values without invoking `equal`. Returns false for a presence mismatch without invoking `equal`. For two Some values, invokes `equal` exactly once, with `a`'s value first, and returns its result.

All operations in Sections 3.3–3.5 are `O(1)`. Apart from `Ptr` and allocations caused by callbacks or escaping values, they target zero allocations.

### 3.6 JSON integration

#### `IsZero`

```go
func (o Optional[T]) IsZero() bool
```

Returns true exactly when `o` is None. This is the `encoding/json` `omitzero` definition and is also available to other standard-library conventions that recognize `IsZero`.

#### `MarshalJSON`

```go
func (o Optional[T]) MarshalJSON() ([]byte, error)
```

For None, returns the bytes `null` and nil error. For Some, delegates to `encoding/json.Marshal` for the stored value and returns its bytes and error. It must not wrap Some in an object or array.

#### `UnmarshalJSON`

```go
func (o *Optional[T]) UnmarshalJSON(data []byte) error
```

If the receiver is nil, a direct call returns a non-nil error without examining `data` and does not panic. `encoding/json.Unmarshal` retains its own standard invalid-target behavior and may reject a nil target before invoking the method.

After ignoring JSON whitespace, the exact token `null` sets the receiver to None and returns nil without invoking a JSON unmarshaler on `T`.

For every other input, the method must decode into a fresh zero-valued temporary `T` using `encoding/json.Unmarshal`. On success it replaces the receiver with Some of that temporary. On error it returns the original error and leaves the receiver unchanged. Decoding replaces rather than merges with a previously present value at the Optional wrapper level.

The required representation matrix is:

| Go state | JSON output | Decode result |
| --- | --- | --- |
| None | `null` | None |
| Some zero non-nil value | encoding of the value | Some(decoded value) |
| `Some[*T](nil)` | `null` | None |
| `Some[[]T](nil)` | `null` | None |
| `Some[any](nil)` | `null` | None |
| Some non-nil value | encoding of the value | Some(decoded value) |

Presence for a present nil cannot round-trip and must not be claimed to do so.

For a struct field `O Optional[T]` under the Go 1 `encoding/json` package:

| Field tag and state | Required behavior |
| --- | --- |
| no omission option, None | field is emitted as `null` |
| `omitempty`, None | field is emitted as `null` |
| `omitzero`, None | field is omitted |
| `omitzero`, Some(zero) | field is emitted |
| `omitzero`, Some(nil) | field is emitted as `null` |

A missing object member leaves a zero-valued destination field as None. Therefore missing and explicit `null` are not distinguished when decoding a newly zero-valued containing struct.

## 4. Package `stream`

Import path:

```go
import "github.com/imbrooklyn/shuttle/stream"
```

The package imports `optional` for `FilterMap` and optional-returning terminals. It may use `cmp`, `iter`, and other standard-library implementation packages. It must not use reflection for typed transformation or conversion.

### 4.1 Complete public declaration inventory

```go
package stream

import (
    "cmp"
    "iter"

    "github.com/imbrooklyn/shuttle/optional"
)

type Stream[T any] struct {
    // unexported
}

type Pair[A, B any] struct {
    First  A
    Second B
}

type Group[K comparable, V any] struct {
    Key    K
    Values []V
}

func Empty[T any]() Stream[T]
func Of[T any](values ...T) Stream[T]
func FromSlice[T any](values []T) Stream[T]
func FromSeq[T any](seq iter.Seq[T]) Stream[T]
func FromSeq2[A, B any](seq iter.Seq2[A, B]) Stream[Pair[A, B]]
func ToSeq2[A, B any](s Stream[Pair[A, B]]) iter.Seq2[A, B]

func Range(start, end int) Stream[int]
func RangeStep(start, end, step int) Stream[int]
func Repeat[T any](value T) Stream[T]
func RepeatN[T any](value T, n int) Stream[T]
func Iterate[T any](seed T, next func(T) T) Stream[T]
func Generate[T any](next func() T) Stream[T]

func Enumerate[T any](s Stream[T]) Stream[Pair[int, T]]
func Zip[A, B any](left Stream[A], right Stream[B]) Stream[Pair[A, B]]
func Chunk[T any](s Stream[T], size int) Stream[[]T]
func Window[T any](s Stream[T], size int) Stream[[]T]
func WindowStep[T any](s Stream[T], size, step int) Stream[[]T]

func Distinct[T comparable](s Stream[T]) Stream[T]
func Contains[T comparable](s Stream[T], value T) bool
func Sorted[T cmp.Ordered](s Stream[T]) Stream[T]
func Min[T cmp.Ordered](s Stream[T]) optional.Optional[T]
func Max[T cmp.Ordered](s Stream[T]) optional.Optional[T]

func (s Stream[T]) Seq() iter.Seq[T]

func (s Stream[T]) Map[R any](fn func(T) R) Stream[R]
func (s Stream[T]) FlatMap[R any](fn func(T) Stream[R]) Stream[R]
func (s Stream[T]) Filter(predicate func(T) bool) Stream[T]
func (s Stream[T]) FilterMap[R any](fn func(T) optional.Optional[R]) Stream[R]
func (s Stream[T]) Inspect(fn func(T)) Stream[T]
func (s Stream[T]) Take(n int) Stream[T]
func (s Stream[T]) Skip(n int) Stream[T]
func (s Stream[T]) TakeWhile(predicate func(T) bool) Stream[T]
func (s Stream[T]) SkipWhile(predicate func(T) bool) Stream[T]
func (s Stream[T]) Scan[A any](initial A, fn func(A, T) A) Stream[A]
func (s Stream[T]) Concat(others ...Stream[T]) Stream[T]

func (s Stream[T]) DistinctBy[K comparable](key func(T) K) Stream[T]
func (s Stream[T]) SortedFunc(compare func(T, T) int) Stream[T]
func (s Stream[T]) Reverse() Stream[T]

func (s Stream[T]) Collect() []T
func (s Stream[T]) AppendTo[S ~[]T](dst S) S
func (s Stream[T]) Count() int
func (s Stream[T]) First() optional.Optional[T]
func (s Stream[T]) Last() optional.Optional[T]
func (s Stream[T]) Find(predicate func(T) bool) optional.Optional[T]
func (s Stream[T]) At(index int) optional.Optional[T]
func (s Stream[T]) Any(predicate func(T) bool) bool
func (s Stream[T]) All(predicate func(T) bool) bool
func (s Stream[T]) None(predicate func(T) bool) bool
func (s Stream[T]) ForEach(fn func(T))
func (s Stream[T]) ForEachErr(fn func(T) error) error
func (s Stream[T]) Reduce[A any](initial A, fn func(A, T) A) A
func (s Stream[T]) ReduceFirst(fn func(T, T) T) optional.Optional[T]
func (s Stream[T]) MinFunc(compare func(T, T) int) optional.Optional[T]
func (s Stream[T]) MaxFunc(compare func(T, T) int) optional.Optional[T]
func (s Stream[T]) MinBy[K cmp.Ordered](key func(T) K) optional.Optional[T]
func (s Stream[T]) MaxBy[K cmp.Ordered](key func(T) K) optional.Optional[T]
func (s Stream[T]) ToMap[K comparable, V any](key func(T) K, value func(T) V) map[K]V
func (s Stream[T]) ToMapWith[K comparable, V any](key func(T) K, value func(T) V, merge func(V, V) V) map[K]V
func (s Stream[T]) GroupBy[K comparable](key func(T) K) []Group[K, T]
```

No `Stream2`, mutable builder, collector interface, `Sum`, error-state method, or concurrency method is public in v1.

`Enumerate`, `Zip`, `Chunk`, `Window`, and `WindowStep` are package functions under the validated Go 1.27 toolchain. A method returning `Stream[Pair[int,T]]`, `Stream[Pair[T,U]]`, or `Stream[[]T]` recursively instantiates the receiver method set with a structurally expanded `T` and is rejected by `go1.27rc3` as an `instantiation cycle`. Go issue [#80172](https://go.dev/issue/80172) tracks this as an overly conservative compiler check targeted after Go 1.27. The equivalent package functions compile. They must not be replaced with reflection, `any`, or type-inference tricks.

The stable-migration gate must retry the direct declarations. If final Go 1.27 accepts them, method versus function shape requires a pre-v1 API review and synchronized specification change; no aliases are added automatically. If stable retains the rejection, the declarations above remain the authoritative v1 API required to support Go 1.27.

### 4.2 Core types

#### `Stream`

```go
type Stream[T any] struct { /* unexported */ }
```

The zero value is a valid empty Stream. A Stream is a lazy ordered sequence descriptor, not stored elements. Copying it copies the descriptor and never clones or rewinds its source.

`Stream[T]` is not comparable. It has no exported fields and no public mutation methods.

#### `Pair`

```go
type Pair[A, B any] struct {
    First  A
    Second B
}
```

`Pair` holds two shallow values. It is comparable exactly when both component types are comparable. Field names and order are frozen for v1.

#### `Group`

```go
type Group[K comparable, V any] struct {
    Key    K
    Values []V
}
```

`Group` is the ordered output record of `GroupBy`. `Values` is a stable, caller-owned slice after `GroupBy` returns. Field names and order are frozen for v1.

### 4.3 Sequence protocol

#### `Seq`

```go
func (s Stream[T]) Seq() iter.Seq[T]
```

Returns a non-nil `iter.Seq[T]`. For a zero or empty Stream, invoking the returned sequence yields no values. Calling `Seq` alone does not traverse the source.

The returned sequence must:

1. call `yield` once per emitted value in encounter order;
2. return immediately when `yield` returns false;
3. never call `yield` again after false; and
4. propagate early termination to upstream.

It must call downstream `yield` serially. `FromSeq` assumes the same serial-call discipline from its source.

Calling a returned sequence more than once inherits source replay behavior. `Seq` must not add caching or a single-use guard.

### 4.4 Finite and iterator sources

#### `Empty`

```go
func Empty[T any]() Stream[T]
```

Returns an empty reusable Stream equivalent to the zero value. Construction and traversal use `O(1)` time and memory.

#### `Of`

```go
func Of[T any](values ...T) Stream[T]
```

Makes a shallow snapshot of the variadic slice during construction and returns a reusable Stream over that snapshot. Later replacement of elements in a slice passed as `values...` must not affect output. Referenced data inside elements remains aliased.

Construction uses `O(len(values))` time and memory. Traversal is lazy, ordered, and `O(n)` time with `O(1)` traversal state. With no arguments, explicit type arguments are required and the result is empty.

#### `FromSlice`

```go
func FromSlice[T any](values []T) Stream[T]
```

Captures the supplied slice header without copying its backing array. The captured length and range are fixed at construction. Elements are read during each traversal, so replacing an element in that range is visible to later traversal. Reslicing the caller's variable does not change the captured length. A nil slice is empty.

Construction is `O(1)` with no required allocation. Traversal is reusable, ordered, `O(n)`, and uses `O(1)` state. Concurrent mutation and traversal have ordinary Go data-race behavior.

#### `FromSeq`

```go
func FromSeq[T any](seq iter.Seq[T]) Stream[T]
```

Wraps `seq` without caching. A nil sequence is normalized to an empty Stream. Encounter order, reuse or single-use behavior, blocking, side effects, and source-owned cleanup otherwise remain those of `seq`.

The wrapper must propagate false from downstream to `seq`. Construction is `O(1)`.

#### `FromSeq2`

```go
func FromSeq2[A, B any](seq iter.Seq2[A, B]) Stream[Pair[A, B]]
```

Lazily converts each `yield(a, b)` call to `Pair[A,B]{First: a, Second: b}`. A nil sequence is empty. It preserves order, early termination, and source replay behavior and performs no per-element heap allocation solely for the Pair value.

#### `ToSeq2`

```go
func ToSeq2[A, B any](s Stream[Pair[A, B]]) iter.Seq2[A, B]
```

Returns a non-nil sequence that lazily yields `pair.First, pair.Second`. It preserves order, early termination, and source replay behavior. It is a package function because a method cannot constrain the receiver to `Stream[Pair[A,B]]` and infer both component types.

### 4.5 Numeric and infinite sources

#### `Range`

```go
func Range(start, end int) Stream[int]
```

Returns the reusable half-open ascending sequence `start, start+1, ...` while values are less than `end`. When `start >= end`, it is empty. `end` is never emitted. The implementation must terminate without signed wraparound at integer boundaries.

#### `RangeStep`

```go
func RangeStep(start, end, step int) Stream[int]
```

Returns a reusable half-open arithmetic sequence:

- for `step > 0`, yields values beginning at `start` while each value is `< end`;
- for `step < 0`, yields values beginning at `start` while each value is `> end`;
- a direction inconsistent with the bounds yields an empty Stream; and
- `step == 0` panics immediately.

The current value is emitted before adding `step`. If the next addition would overflow `int`, traversal terminates instead of wrapping. The end bound remains exclusive.

Both range constructors use `O(1)` construction and traversal memory and `O(n)` traversal time.

#### `Repeat`

```go
func Repeat[T any](value T) Stream[T]
```

Returns an infinite reusable Stream that yields a shallow copy of `value` for every demand. It is IS with `O(1)` state.

#### `RepeatN`

```go
func RepeatN[T any](value T, n int) Stream[T]
```

Returns a reusable Stream containing exactly `n` shallow copies of `value`. Zero is empty; a negative count panics immediately. It is finite, lazy, and uses `O(1)` traversal state.

#### `Iterate`

```go
func Iterate[T any](seed T, next func(T) T) Stream[T]
```

Returns an infinite Stream. Each traversal emits `seed` first. After an emitted value is accepted and another value is demanded, it invokes `next` exactly once with the prior value to obtain the next value. If downstream stops after the seed, `next` is not invoked.

The current value is local to each traversal. The callback object and any state it captures are shared. The operator is IS with `O(1)` state.

#### `Generate`

```go
func Generate[T any](next func() T) Stream[T]
```

Returns an infinite Stream that invokes `next` exactly once immediately before each emitted value. It does not invoke `next` at construction or when upstream is bypassed by `Take(0)`.

The same callback is shared by every traversal. Shuttle does not reset captured generator state, so replay and concurrent safety depend on the callback. The operator is IS with `O(1)` Shuttle state.

### 4.6 Stateless and bounded-state transformations

All transformations in this section are lazy at construction and preserve source replay behavior.

#### `Map`

```go
func (s Stream[T]) Map[R any](fn func(T) R) Stream[R]
```

Invokes `fn` exactly once for every upstream element consumed and yields the result, including zero and typed nil results. It preserves encounter order, propagates early termination, is IS, and uses `O(1)` traversal state.

#### `FlatMap`

```go
func (s Stream[T]) FlatMap[R any](fn func(T) Stream[R]) Stream[R]
```

For each consumed outer element, invokes `fn` exactly once and completely traverses the returned inner Stream before requesting the next outer element. Empty inner Streams contribute nothing. Output order is outer encounter order followed by each inner encounter order.

Downstream false must stop the active inner Stream and then the outer Stream. An infinite inner Stream prevents later outer elements from being reached, but downstream can still terminate it. Time is `O(n + q)`, only one inner traversal is active, and Shuttle state is `O(1)` excluding source/inner state. Classification: IS for streaming and bounded Shuttle state.

#### `Filter`

```go
func (s Stream[T]) Filter(predicate func(T) bool) Stream[T]
```

Invokes `predicate` once per consumed upstream value and yields the value only when true. It preserves order, uses `O(1)` state, propagates early termination, and is IS. It may consume indefinitely without output when no value matches.

#### `FilterMap`

```go
func (s Stream[T]) FilterMap[R any](fn func(T) optional.Optional[R]) Stream[R]
```

Invokes `fn` once per consumed upstream value. A None result is discarded; a Some result yields its stored value, including a present typed nil. It must inspect presence through the Optional API and must not interpret the zero value of `R` as absent. It preserves order, uses `O(1)` state, and is IS.

#### `Inspect`

```go
func (s Stream[T]) Inspect(fn func(T)) Stream[T]
```

For every consumed upstream value, invokes `fn` exactly once before offering that same value downstream. It preserves the value and order, uses `O(1)` state, and is IS. A value rejected by the immediate downstream yield has already been inspected.

#### `Take`

```go
func (s Stream[T]) Take(n int) Stream[T]
```

Yields at most the first `n` upstream values. Zero produces an empty traversal without invoking upstream. A negative count panics immediately.

After yielding the nth value, `Take` returns false upstream without requesting an additional value, even if the source would naturally end at exactly `n`. It is IS, order-preserving, short-circuiting, and uses `O(1)` state.

#### `Skip`

```go
func (s Stream[T]) Skip(n int) Stream[T]
```

Consumes and discards the first `n` upstream values, then yields all later values. Zero is an identity transformation. A negative count panics immediately. It is IS, order-preserving, and uses `O(1)` state.

#### `TakeWhile`

```go
func (s Stream[T]) TakeWhile(predicate func(T) bool) Stream[T]
```

Tests values in order and yields the longest prefix for which `predicate` is true. It consumes and tests the first failing value, discards it, and stops upstream immediately. If no value fails, it follows upstream to exhaustion. It is IS with `O(1)` state.

#### `SkipWhile`

```go
func (s Stream[T]) SkipWhile(predicate func(T) bool) Stream[T]
```

Tests and discards the longest prefix for which `predicate` is true. The first value for which it is false is yielded, and the predicate is never called again during that traversal. It is IS, order-preserving, and uses `O(1)` state.

#### `Scan`

```go
func (s Stream[T]) Scan[A any](initial A, fn func(A, T) A) Stream[A]
```

Maintains an accumulator initialized to a shallow copy of `initial`. For every upstream value, computes `acc = fn(acc, value)` and then yields the new accumulator. The initial value itself is not emitted. Empty input produces empty output, not one initial value.

It emits exactly one value per consumed upstream value, preserves order, is IS, and uses `O(1)` state apart from the accumulator value.

Emitted accumulators are shallow values, not snapshots. If `A` contains a slice, map, pointer, or other reference-like value and `fn` reuses it, multiple emitted accumulators may alias that referenced storage.

Each traversal starts from another shallow copy of the captured `initial`. Reference-like storage reachable from `initial` is therefore shared across traversals and may carry mutations made by an earlier traversal.

#### `Enumerate`

```go
func Enumerate[T any](s Stream[T]) Stream[Pair[int, T]]
```

Yields `Pair[int,T]{First: index, Second: value}` with zero-based consecutive indices. It preserves order and uses `O(1)` state. If a traversal demands a value after index `math.MaxInt` has been emitted, it panics rather than wrapping the index. It is IS subject to the finite `int` index space.

#### `Concat`

```go
func (s Stream[T]) Concat(others ...Stream[T]) Stream[T]
```

Yields every receiver value, then every value of each `others` Stream in argument order. It must make a shallow snapshot of the variadic Stream descriptor slice at construction so later replacement of entries in a passed slice cannot change the pipeline.

It does not begin a later Stream until every earlier Stream is exhausted. Downstream false stops the active Stream and prevents later Streams from starting. It is IS, order-preserving, and uses `O(1)` traversal state plus the `O(len(others))` descriptor snapshot.

#### `Zip`

```go
func Zip[A, B any](left Stream[A], right Stream[B]) Stream[Pair[A, B]]
```

Pairs values at equal encounter positions and stops when either side ends. It yields no padding or partial pair. Pair order is `left` value in `First`, `right` value in `Second`.

For each pair, Zip requests the left value first and the right value second. Therefore:

- if `left` ends first, Zip does not request an unmatched value from `right`;
- if `right` ends first, Zip has already consumed and discards exactly one unmatched left value; and
- when downstream rejects a complete pair, no later value is requested from either side.

If `left` emits no value, Zip must not invoke the right sequence. The right pull iterator must therefore be created lazily after the first left value is obtained.

The left sequence is the push-style driver. The implementation must convert the right sequence with `iter.Pull`, arrange deferred cleanup for its `stop` function, and never call its `next` and `stop` concurrently. It must return false directly to the left sequence whenever the right side ends or downstream stops. The right pull iterator must be stopped on normal exhaustion, shorter-side termination, downstream false, and panic unwinding; the left source must be allowed to return and run its own cleanup. Zip starts no Shuttle goroutine.

Zip is IS, ordered by position, `O(min(n, m))` in emitted pairs, and uses bounded `O(1)` traversal state plus standard pull-iterator setup.

### 4.7 Stateful and barrier transformations

#### `DistinctBy`

```go
func (s Stream[T]) DistinctBy[K comparable](key func(T) K) Stream[T]
```

For each consumed value, invokes `key` exactly once. It yields the first value for each key and discards later values with an equal key. Output preserves the encounter order of first occurrences.

The key set is allocated per traversal. Expected time is `O(n)` and additional memory is `O(u)`. Downstream false stops immediately. Classification: C, because an infinite input can grow the key set without bound or consume an infinite suffix of already-seen keys without output.

#### `Distinct`

```go
func Distinct[T comparable](s Stream[T]) Stream[T]
```

Is the constrained identity-key form of `DistinctBy`. It has the same ordering, laziness, early-termination, complexity, and infinite-input behavior. It is package-level because `Stream[T any]` cannot strengthen `T` to `comparable` on one method.

#### `Chunk`

```go
func Chunk[T any](s Stream[T], size int) Stream[[]T]
```

Partitions encounter order into consecutive non-overlapping chunks. Every full chunk has length `size`. If upstream ends with a non-empty remainder, it emits one final partial chunk. Empty input emits no chunks. A non-positive size panics immediately.

Each emitted slice must have a newly allocated backing array that is distinct from all other output slices and internal working storage, and must satisfy `cap(chunk) == len(chunk)`. A caller may retain, append to, or mutate a chunk after requesting later values without changing any other emitted chunk.

The operator consumes no values beyond the end of the chunk currently being emitted. It is IS and order-preserving. Total copying is `O(n)`, traversal working memory is `O(size)`, and output allocates once per emitted chunk.

Example:

```text
1 2 3 4 5 6 7  --Chunk(3)-->  [1 2 3] [4 5 6] [7]
```

#### `Window`

```go
func Window[T any](s Stream[T], size int) Stream[[]T]
```

Is exactly `WindowStep(s, size, 1)`. It emits full sliding windows only. A non-positive size panics immediately.

Example:

```text
1 2 3 4 5  --Window(3)-->  [1 2 3] [2 3 4] [3 4 5]
```

#### `WindowStep`

```go
func WindowStep[T any](s Stream[T], size, step int) Stream[[]T]
```

For a finite conceptual input `x` of length `n`, emits exactly the slices:

```text
x[start : start+size]
```

for `start = 0, step, 2*step, ...` while `start+size <= n`. A partial final window is never emitted. If `n < size`, output is empty. `step < size` produces overlap, `step == size` is equivalent in boundaries to full chunks without a partial remainder, and `step > size` skips gaps between windows.

`size` and `step` must both be positive or the function panics immediately.

Every emitted window must have its own backing array, distinct from every other output and internal storage, with `cap(window) == len(window)`. The caller may retain and mutate it safely. The operator must not consume gap or next-window values until downstream accepts the current window and requests progress.

It is IS and preserves order within and among window starts. Working memory is `O(size)`. Time is `O(n + w*size)` because every emitted owned window is copied, and output allocates once per window.

#### `SortedFunc`

```go
func (s Stream[T]) SortedFunc(compare func(T, T) int) Stream[T]
```

Returns a construction-lazy barrier Stream. At traversal it consumes all upstream values into a fresh buffer, stably sorts them, and only then emits them. `compare(a, b)` must return a negative value when `a < b`, zero when equivalent for sorting, and a positive value when `a > b`; it must define a strict weak ordering. Shuttle does not validate the ordering.

If `compare` violates that contract, the output order is unspecified. Any comparator panic propagates.

Elements comparing zero retain their original encounter order. Comparator invocation count and order are unspecified and may change across compatible releases. Empty input emits nothing. Each traversal recollects and resorts the source.

Classification: F. It uses `O(n)` retained buffer memory. It requires `O(n log n)` comparisons; an in-place stable implementation may perform up to `O(n log² n)` element movements. It does not mutate a slice owned by the source.

#### `Sorted`

```go
func Sorted[T cmp.Ordered](s Stream[T]) Stream[T]
```

Is the package-level natural-order form of `SortedFunc`, using `cmp.Compare`. It is stable and has the same barrier, replay, memory, and infinite-input behavior. In particular, floating-point NaN ordering follows `cmp.Compare`, not raw `<` alone.

#### `Reverse`

```go
func (s Stream[T]) Reverse() Stream[T]
```

Returns a construction-lazy barrier Stream. At traversal it consumes all upstream values into a fresh buffer, then emits them in exact reverse encounter order. It does not mutate source-owned storage. Empty input remains empty, and every traversal recollects the source.

Classification: F. Time and memory are `O(n)`. It cannot emit before upstream ends.

### 4.8 Terminal operations

Every operation in this section starts one traversal immediately.

#### `Collect`

```go
func (s Stream[T]) Collect() []T
```

Consumes all values into a newly built slice in encounter order. Empty input returns a nil slice, matching `slices.Collect`. The returned backing array is not internal reusable Stream state. Time and returned memory are `O(n)`. Classification: F.

#### `AppendTo`

```go
func (s Stream[T]) AppendTo[S ~[]T](dst S) S
```

Appends all values in encounter order using ordinary `append` semantics and returns the resulting slice with its named slice type preserved. Existing `dst` elements remain the prefix. It may reuse `dst`'s backing array. Empty input returns `dst` unchanged and preserves nilness. Time is `O(n)` and additional allocation depends on destination capacity. Classification: F.

#### `Count`

```go
func (s Stream[T]) Count() int
```

Consumes all values and returns their count. Empty input returns zero. It uses `O(1)` memory. If accepting another element would overflow `int`, it panics instead of wrapping. Classification: F.

#### `First`

```go
func (s Stream[T]) First() optional.Optional[T]
```

Returns Some of the first value and stops upstream immediately, or None for empty input. It consumes at most one value, uses `O(1)` memory, and is IS.

#### `Last`

```go
func (s Stream[T]) Last() optional.Optional[T]
```

Consumes all input and returns Some of the last value, or None for empty input. It uses `O(1)` memory and is F.

#### `Find`

```go
func (s Stream[T]) Find(predicate func(T) bool) optional.Optional[T]
```

Tests values in order and returns Some of the first matching value, stopping upstream immediately. It returns None only after empty input or complete exhaustion with no match. It uses `O(1)` memory and is C.

#### `At`

```go
func (s Stream[T]) At(index int) optional.Optional[T]
```

For `index >= 0`, returns Some of the zero-based element at that position and stops without requesting the next value. It returns None if upstream ends first. For `index < 0`, it returns None without invoking upstream. It uses `O(1)` memory and is IS for every finite non-negative index.

#### `Any`

```go
func (s Stream[T]) Any(predicate func(T) bool) bool
```

Returns true at the first value for which `predicate` is true and stops upstream. Returns false after exhaustion without a match. Empty input returns false. It is C with `O(1)` memory.

#### `All`

```go
func (s Stream[T]) All(predicate func(T) bool) bool
```

Returns false at the first value for which `predicate` is false and stops upstream. Returns true after complete successful exhaustion. Empty input returns true. It is C with `O(1)` memory.

#### `None`

```go
func (s Stream[T]) None(predicate func(T) bool) bool
```

Returns false at the first value for which `predicate` is true and stops upstream. Returns true after exhaustion with no match. Empty input returns true. It is logically `!Any(predicate)` but is included because it directly expresses a common quantifier without building another Stream. It is C with `O(1)` memory.

#### `Contains`

```go
func Contains[T comparable](s Stream[T], value T) bool
```

Returns true at the first element equal to `value` by `==` and stops upstream; otherwise false after exhaustion. It is package-level because of the receiver-element constraint. Arbitrary equivalence uses `Any`. It is C with `O(1)` memory.

If `T` is an interface type and either operand contains a dynamically non-comparable value, equality has ordinary Go behavior and panics.

#### `ForEach`

```go
func (s Stream[T]) ForEach(fn func(T))
```

Invokes `fn` once for every value in encounter order and returns after exhaustion. Empty input does nothing. It uses `O(1)` memory and is F for completion.

#### `ForEachErr`

```go
func (s Stream[T]) ForEachErr(fn func(T) error) error
```

Invokes `fn` in encounter order. Nil errors continue. The first non-nil error stops upstream immediately and is returned unchanged. Returns nil after successful exhaustion, including empty input. It has no hidden error state, uses `O(1)` memory, and is C.

#### `Reduce`

```go
func (s Stream[T]) Reduce[A any](initial A, fn func(A, T) A) A
```

Starts with a shallow copy of `initial`. For every value in encounter order, assigns `acc = fn(acc, value)`. Returns the final accumulator. Empty input returns `initial` without invoking `fn`. It uses `O(1)` accumulator state and is F.

#### `ReduceFirst`

```go
func (s Stream[T]) ReduceFirst(fn func(T, T) T) optional.Optional[T]
```

Returns None for empty input. Otherwise uses the first element as the accumulator, invokes `fn(acc, value)` for each later element in order, and returns Some of the final accumulator. A singleton does not invoke `fn`. It uses `O(1)` state and is F.

#### `MinFunc` and `MaxFunc`

```go
func (s Stream[T]) MinFunc(compare func(T, T) int) optional.Optional[T]
func (s Stream[T]) MaxFunc(compare func(T, T) int) optional.Optional[T]
```

Consume all input and return None when empty. `MinFunc` retains a candidate `c` unless `compare(value, c) < 0`; `MaxFunc` replaces it only when `compare(value, c) > 0`. Therefore the first encountered value wins comparison ties.

The comparator contract matches `SortedFunc`. It is invoked exactly once for each element after the first, in encounter order, with the new value first and candidate second. Both use `O(1)` memory, `O(n)` time, and are F.

#### `MinBy` and `MaxBy`

```go
func (s Stream[T]) MinBy[K cmp.Ordered](key func(T) K) optional.Optional[T]
func (s Stream[T]) MaxBy[K cmp.Ordered](key func(T) K) optional.Optional[T]
```

Consume all input and return None when empty. They invoke `key` exactly once per consumed element, retain both the candidate and its key, compare keys with `cmp.Compare`, and return the original element. The first element wins equal-key ties. Floating-point key NaN behavior follows `cmp.Compare`.

Both use `O(1)` memory, `O(n)` time, and are F.

#### `Min` and `Max`

```go
func Min[T cmp.Ordered](s Stream[T]) optional.Optional[T]
func Max[T cmp.Ordered](s Stream[T]) optional.Optional[T]
```

Are package-level natural-order extrema using `cmp.Compare`. They return None for empty input and the first encountered value on a comparison tie. They use `O(1)` memory, `O(n)` time, and are F.

#### `ToMap`

```go
func (s Stream[T]) ToMap[K comparable, V any](key func(T) K, value func(T) V) map[K]V
```

Allocates a new non-nil map, even for empty input. For each element it invokes `key` exactly once, then `value` exactly once, and assigns `result[k] = v`. A later element with an equal key replaces the earlier value. Map iteration order is unspecified.

Expected time is `O(n)`, returned memory is `O(u)`, and classification is F.

#### `ToMapWith`

```go
func (s Stream[T]) ToMapWith[K comparable, V any](
    key func(T) K,
    value func(T) V,
    merge func(V, V) V,
) map[K]V
```

Allocates a new non-nil map. For each element it invokes `key` exactly once and then `value` exactly once. For a first key occurrence it stores the incoming value without invoking `merge`. For a duplicate, it invokes `merge(existing, incoming)` exactly once and stores the result. Subsequent duplicates receive the accumulated stored result as `existing`.

Expected time is `O(n)` plus merge work, returned memory is `O(u)`, and classification is F.

#### `GroupBy`

```go
func (s Stream[T]) GroupBy[K comparable](key func(T) K) []Group[K, T]
```

Consumes all input. It invokes `key` exactly once per element. The result contains one Group per distinct key, ordered by the key's first encounter. Each Group's values retain source encounter order.

Empty input returns a nil slice. For non-empty input, the returned Group slice and every `Values` slice are stable and owned by the caller; no two non-empty groups share a mutable backing array, and no Values slice aliases internal reusable state. Values remain shallow copies. Capacities other than the non-aliasing guarantee are unspecified.

Expected time is `O(n)`, returned and indexing memory is `O(n + g)`, and classification is F.

### 4.9 Unified operator matrix

The matrix summarizes normative properties. “SC” means the operation propagates downstream early termination or itself returns early.

| Operation | Evaluation | Time | Additional memory | SC | Order | Infinite |
| --- | --- | --- | --- | :---: | --- | :---: |
| `Map` | incremental | `O(n)` + callback | `O(1)` | yes | preserved | IS |
| `FlatMap` | incremental | `O(n+q)` + callback | `O(1)` Shuttle state | yes | outer then inner | IS |
| `Filter` | incremental | `O(n)` + predicate | `O(1)` | yes | preserved subset | IS |
| `FilterMap` | incremental | `O(n)` + callback | `O(1)` | yes | preserved subset | IS |
| `Inspect` | incremental | `O(n)` + callback | `O(1)` | yes | preserved | IS |
| `Take(k)` | incremental | `O(min(n,k))` | `O(1)` | yes | prefix | IS |
| `Skip(k)` | incremental | `O(n)` | `O(1)` | yes | suffix | IS |
| `TakeWhile` | incremental | prefix plus first failure | `O(1)` | yes | prefix | IS |
| `SkipWhile` | incremental | `O(n)` | `O(1)` | yes | suffix | IS |
| `Scan` | incremental | `O(n)` + callback | `O(1)` | yes | preserved positions | IS |
| `Enumerate` | incremental | `O(n)` | `O(1)` | yes | preserved | IS |
| `Concat` | incremental | consumed total | `O(len(others))` descriptors + `O(1)` traversal | yes | stream order | IS |
| `Zip` | incremental | emitted pairs | `O(1)` + pull setup | yes | positional | IS |
| `DistinctBy` / `Distinct` | incremental | expected `O(n)` | `O(u)` | yes | first occurrences | C |
| `Chunk(k)` | incremental by chunk | `O(n)` | `O(k)` + owned outputs | yes | preserved | IS |
| `WindowStep(k, step)` | incremental by window | `O(n+w*k)` | `O(k)` + owned outputs | yes | start order | IS |
| `SortedFunc` / `Sorted` | lazy barrier | `O(n log n)` comparisons | `O(n)` | downstream only after barrier | stable sorted | F |
| `Reverse` | lazy barrier | `O(n)` | `O(n)` | downstream only after barrier | reversed | F |
| `Collect` | terminal | `O(n)` | `O(n)` result | no | preserved | F |
| `AppendTo` | terminal | `O(n)` | capacity-dependent | no | preserved | F |
| `Count` | terminal | `O(n)` | `O(1)` | no | n/a | F |
| `First` | terminal | `O(1)` element | `O(1)` | yes | first | IS |
| `Last` | terminal | `O(n)` | `O(1)` | no | last | F |
| `Find` | terminal | through first match | `O(1)` | yes | first match | C |
| `At(k)` | terminal | `O(k+1)` bounded by source | `O(1)` | yes | positional | IS |
| `Any` / `All` / `None` | terminal | through decisive value | `O(1)` | yes | encounter order | C |
| `Contains` | terminal | through first equality | `O(1)` | yes | encounter order | C |
| `ForEach` | terminal | `O(n)` + callback | `O(1)` | no | encounter order | F |
| `ForEachErr` | terminal | through first error | `O(1)` | yes | encounter order | C |
| `Reduce` / `ReduceFirst` | terminal | `O(n)` + callback | `O(1)` | no | left fold | F |
| extrema | terminal | `O(n)` | `O(1)` | no | first tie | F |
| `ToMap` / `ToMapWith` | terminal | expected `O(n)` | `O(u)` result | no | later/merged duplicate | F |
| `GroupBy` | terminal | expected `O(n)` | `O(n+g)` result | no | first-key and value order | F |

### 4.10 Iterator lifetime and cleanup requirements

An implementation must include instrumented tests proving all of the following:

- every adapter returns false upstream immediately after downstream false;
- `Take(0)` does not invoke its source;
- `Take(n)` does not request element `n+1`;
- `TakeWhile` consumes exactly through its first failing element;
- `FlatMap` stops both the active inner traversal and the outer traversal;
- `Concat` never starts a later source after early termination;
- Zip does not invoke the right source when the left source is empty and consumes exactly one unmatched left value when the right source ends first;
- Zip stops its right pull iterator and unwinds its left push iterator when either side ends, downstream stops, or a panic unwinds;
- Zip never calls the right pull iterator's `next` and `stop` simultaneously;
- a source `defer` executes after every short-circuiting terminal; and
- no Shuttle-created goroutine can leak, because core implementation creates none.

If an operator uses `iter.Pull` beyond the required Zip implementation, it must satisfy the same stop and synchronization rules and must be justified in code review.

## 5. Package `predicate`

Import path:

```go
import "github.com/imbrooklyn/shuttle/predicate"
```

The package is independent and must not import `optional` or `stream`. It may use `reflect` only within the typed-nil behavior specified in Section 5.5. It must have no third-party runtime dependency.

### 5.1 Complete public declaration inventory

The package must export exactly the following v1 API:

```go
package predicate

type Func[T any] func(T) bool

func (p Func[T]) Not() Func[T]
func (p Func[T]) And(others ...Func[T]) Func[T]
func (p Func[T]) Or(others ...Func[T]) Func[T]

func Always[T any](result bool) Func[T]
func Equal[T comparable](want T) Func[T]
func EqualFunc[T any](want T, equal func(T, T) bool) Func[T]
func On[A, B any](project func(A) B, predicate Func[B]) Func[A]

func IsNil[T any](value T) bool
func IsNotNil[T any](value T) bool
```

There are no exported interfaces, struct wrappers, sentinel values, state types, aliases, or helper names such as `AllOf`, `AnyOf`, `Negate`, `Matches`, `Test`, `Apply`, `Compose`, `Contramap`, or `Equals` in v1.

### 5.2 `Func` type and common composition contract

```go
type Func[T any] func(T) bool
```

`Func[T]` is a defined function type whose underlying type is exactly `func(T) bool`. A value is therefore directly assignable to an unnamed `func(T) bool` parameter, including `optional.Optional[T].Filter` and `stream.Stream[T].Filter`. Ordinary unnamed function values with the matching signature are assignable where `Func[T]` is required.

The zero value of `Func[T]` is a nil function. It has no implicit true, false, identity, or disabled meaning. Calling it has ordinary Go nil-function behavior and panics. Merely constructing a composition around it must not invoke it or panic.

All `Func` methods are construction-lazy. Every evaluation must be:

- synchronous, sequential, and left to right;
- governed by the exact short-circuit rules of the selected operation;
- free of panic recovery, goroutine creation, result caching, and global locking; and
- free of allocation caused by composition after construction, when reached callbacks themselves do not allocate.

Every evaluation is independent and must invoke every reached predicate anew. Function values and captured state retain normal Go shallow aliasing and concurrency semantics. A composition is safe for concurrent evaluation only when every reached predicate and captured value is safe for concurrent use. Shuttle must not add synchronization around caller-owned state.

For every variadic composition, the supplied predicate descriptor slice must be shallow-snapshotted at construction. Replacing an entry in the caller's slice afterward must not change the composition. Captured objects reachable through the copied function values remain shared.

### 5.3 Composition methods

#### `Not`

```go
func (p Func[T]) Not() Func[T]
```

Returns a predicate that invokes `p` exactly once for each evaluation and returns the logical negation of its result. A panic from `p`, including ordinary invocation of a nil `p`, propagates unchanged.

#### `And`

```go
func (p Func[T]) And(others ...Func[T]) Func[T]
```

For each evaluation, invokes `p` first and then each entry of the construction-time snapshot of `others` in argument order. It returns false immediately after the first false result and returns true only when every reached predicate returns true. With no `others`, it invokes `p` exactly once and returns its result.

A nil predicate skipped after a false result must not panic. A nil receiver or entry that is reached must panic through ordinary nil-function invocation. No later predicate may be invoked after a false result or panic.

#### `Or`

```go
func (p Func[T]) Or(others ...Func[T]) Func[T]
```

For each evaluation, invokes `p` first and then each entry of the construction-time snapshot of `others` in argument order. It returns true immediately after the first true result and returns false only when every reached predicate returns false. With no `others`, it invokes `p` exactly once and returns its result.

A nil predicate skipped after a true result must not panic. A nil receiver or entry that is reached must panic through ordinary nil-function invocation. No later predicate may be invoked after a true result or panic.

### 5.4 Constructors and adapters

#### `Always`

```go
func Always[T any](result bool) Func[T]
```

Returns a predicate that ignores its input and always returns `result`. Construction and evaluation do not invoke user code.

#### `Equal`

```go
func Equal[T comparable](want T) Func[T]
```

Returns a predicate that evaluates `current == want` using normal Go equality. It supports the zero value of every comparable type and performs no reflective or deep equality. When `T` is an interface type and a compared dynamic value is non-comparable, evaluation has ordinary Go behavior and may panic.

#### `EqualFunc`

```go
func EqualFunc[T any](want T, equal func(T, T) bool) Func[T]
```

For every evaluation, invokes `equal` exactly once as `equal(current, want)` and returns its result. Current value first and wanted value second is normative argument order. A nil `equal` callback is accepted during construction and panics by ordinary nil-function invocation only when the returned predicate is evaluated.

#### `On`

```go
func On[A, B any](project func(A) B, predicate Func[B]) Func[A]
```

For every evaluation, invokes `project` exactly once with the current `A`. Only after projection returns, it invokes `predicate` exactly once with the resulting `B` and returns that result. Projection must not be recomputed. A project panic prevents predicate invocation; a predicate panic occurs after exactly one successful projection. Either panic propagates unchanged. Nil callbacks follow the same reached-call rule.

### 5.5 Nil predicates and reflection boundary

#### `IsNil`

```go
func IsNil[T any](value T) bool
```

Returns true exactly when `value` is:

- a nil interface value; or
- a nil channel, function, map, pointer, unsafe pointer, or slice, including a named nilable type or a typed nil stored in an interface.

It returns false for every non-nil value and non-nilable type and must not panic for those inputs.

To preserve typed-nil semantics for `T any`, the implementation may box `value` into an interface and use reflection to inspect its dynamic kind. Reflection is restricted to `IsNil` and must be limited to `reflect.ValueOf`, kind classification, and nil testing. This exception must not be extended to equality, projection, composition, Optional, or Stream. Interface boxing and reflection dispatch are explicit performance costs and may affect escape analysis, but the implementation must not intentionally allocate or cache values.

#### `IsNotNil`

```go
func IsNotNil[T any](value T) bool
```

Returns exactly `!IsNil(value)` for every value. It must not maintain a separate kind table or semantics that can drift from `IsNil`.

Both functions are generic functions, not constructor results. Type inference and argument-context reverse inference must allow matching uses such as:

```go
var keepPointers func(*int) bool = predicate.IsNotNil

present := optional.Some(pointer).Filter(predicate.IsNotNil)
pointers := stream.FromSlice(values).Filter(predicate.IsNil)
```

Explicit instantiation, such as `predicate.IsNil[*int]`, must also compile.

### 5.6 Complexity and interoperability matrix

| API | Construction | Evaluation time | Evaluation allocation | Short-circuit |
| --- | --- | --- | --- | --- |
| `Not` | `O(1)` | one receiver call | zero | n/a |
| `And` | `O(len(others))` snapshot | through first false | zero | first false |
| `Or` | `O(len(others))` snapshot | through first true | zero | first true |
| `Always` | `O(1)` | `O(1)` | zero | n/a |
| `Equal` | `O(1)` | one `==` | zero | n/a |
| `EqualFunc` | `O(1)` | one callback | zero apart from callback | n/a |
| `On` | `O(1)` | one projection plus one predicate | zero apart from callbacks | project panic prevents predicate |
| `IsNil` / `IsNotNil` | n/a | `O(1)` reflection dispatch | no intentional allocation | n/a |

The allocation statements assume values and callbacks do not escape because of caller code. Compiler escape behavior is validated with the target Go toolchain rather than guaranteed across all future compilers.

## 6. Package `comparator`

Import path:

```go
import "github.com/imbrooklyn/shuttle/comparator"
```

Package comparator imports only the standard library. Optional, predicate, and Stream must not import it.

### 6.1 Complete public declaration inventory

The package must export exactly the following v1 API:

```go
package comparator

import "cmp"

type Func[T any] func(left, right T) int

func Ordered[T cmp.Ordered]() Func[T]
func By[T any, K cmp.Ordered](key func(T) K) Func[T]
func ByDescending[T any, K cmp.Ordered](key func(T) K) Func[T]
func On[A, B any](project func(A) B, compare Func[B]) Func[A]
func OnDescending[A, B any](project func(A) B, compare Func[B]) Func[A]

func (c Func[T]) Reverse() Func[T]
func (c Func[T]) Then(others ...Func[T]) Func[T]
func (c Func[T]) ThenBy[K cmp.Ordered](key func(T) K) Func[T]
func (c Func[T]) ThenByDescending[K cmp.Ordered](key func(T) K) Func[T]
func (c Func[T]) ThenOn[K any](project func(T) K, compare Func[K]) Func[T]
func (c Func[T]) ThenOnDescending[K any](project func(T) K, compare Func[K]) Func[T]
```

There are no exported builder types, direction enums, options, aliases, sentinels, or validation errors. In particular, the package must not export `Asc`, `Desc`, `OnAscending`, `ThenOnAscending`, `ByFunc`, `ThenByFunc`, `Compare`, `Comparing`, `ThenComparing`, `Chain`, `Compose`, `Natural`, or `Default`.

### 6.2 `Func` type and common comparison contract

```go
type Func[T any] func(left, right T) int
```

For `result := compare(left, right)`, only the sign is semantically meaningful:

- `result < 0` orders left before right;
- `result == 0` makes left and right equivalent at that ordering level; and
- `result > 0` orders left after right.

Result magnitude is not a public contract unless an individual constructor explicitly delegates to a standard-library function whose result is specified. Callers must supply comparators that define a strict weak ordering. Shuttle does not validate asymmetry, negative-relation transitivity, or equivalence transitivity. Consumer results are unspecified when the ordering is invalid; this does not denote Go language undefined behavior.

The zero value of `Func[T]` is a nil function. Comparator must not reinterpret nil as equality, natural order, an identity, or a direction. Constructing a composition around nil is valid and does not invoke it. Evaluation panics through ordinary nil-function invocation only when execution reaches that descriptor. A descriptor skipped by lexicographic short-circuiting must not panic.

All constructors and methods are construction-lazy. Within one evaluation, reached callbacks run synchronously, serially, and in their specified order. Comparator must not recover a panic, start a goroutine, acquire a global lock, use reflection, cache a key across evaluations, validate results, or retain state proportional to a sorted collection. Every evaluation repeats all callbacks required by its reached path.

Function values and callback-captured references use ordinary shallow Go aliasing. Immutable compositions may be evaluated concurrently only when all reached callbacks, captures, projections, and compared values are safe for concurrent use. Comparator supplies no synchronization for caller-owned state.

### 6.3 Constructors and projection adapters

#### `Ordered`

```go
func Ordered[T cmp.Ordered]() Func[T]
```

Returns the natural ascending comparator implemented by `cmp.Compare(left, right)`. Named ordered types, integer boundaries, strings, floating-point NaNs, and signed zero follow `cmp.Compare` exactly. `Ordered` takes no value argument, so `T` must be explicitly instantiated, such as `Ordered[int]()`. Evaluation adds no allocation; construction has only bounded descriptor cost and may allocate if the instantiated generic function value escapes.

#### `By`

```go
func By[T any, K cmp.Ordered](key func(T) K) Func[T]
```

Returns an ascending ordered-key comparator. Each reached evaluation must:

1. invoke `key(left)` exactly once;
2. invoke `key(right)` exactly once, only after the left call returns; and
3. return `cmp.Compare(leftKey, rightKey)`.

The two projected values may be held locally for that evaluation but must not be cached across evaluations. A left projection panic prevents the right projection; either projection panic prevents comparison. A nil key is accepted at construction and panics when the first reached projection is invoked.

#### `ByDescending`

```go
func ByDescending[T any, K cmp.Ordered](key func(T) K) Func[T]
```

Has the same projection order, count, panic, and caching contract as `By`. After computing left and then right keys, it returns `cmp.Compare(rightKey, leftKey)`. Descending direction must not reverse projection callback order.

#### `On`

```go
func On[A, B any](project func(A) B, compare Func[B]) Func[A]
```

Returns a comparator for arbitrary projected values and custom orderings. Each reached evaluation must:

1. invoke `project(left)` exactly once;
2. invoke `project(right)` exactly once, only after the left call returns; and
3. invoke `compare(leftProjected, rightProjected)` exactly once.

It returns the comparator result without interpreting its magnitude. A projection panic skips every later call; a comparator panic occurs only after both projections return. Nil callbacks follow the same reached-path rule. `On(project, time.Time.Compare)` is the canonical composition for a `time.Time` key because `time.Time` does not satisfy `cmp.Ordered`.

#### `OnDescending`

```go
func OnDescending[A, B any](project func(A) B, compare Func[B]) Func[A]
```

Has the same projection order, count, panic, nil, and caching contract as `On`. It invokes `compare(leftProjected, rightProjected)` exactly once and then maps a negative result to `1`, a positive result to `-1`, and zero to zero. It must not swap comparator arguments or compute `-result`; preserving left-right arguments keeps callback behavior predictable, and sign normalization handles `math.MinInt` without overflow. Descending direction is relative to the ordering represented by `compare`, regardless of how that comparator was constructed.

### 6.4 Composition methods

#### `Reverse`

```go
func (c Func[T]) Reverse() Func[T]
```

Returns a comparator that reverses the complete relation represented by `c`. Each evaluation invokes `c(left, right)` exactly once, without swapping its arguments. It maps a negative result to `1`, a positive result to `-1`, and zero to zero. It must not compute `-result`, because negating `math.MinInt` overflows.

`c.Reverse().Reverse()` must have the same result sign as `c`; nonzero magnitude need not be restored. A nil receiver is accepted during construction and panics only when the returned comparator invokes it. Receiver panics propagate unchanged.

#### `Then`

```go
func (c Func[T]) Then(others ...Func[T]) Func[T]
```

Returns lexicographic composition. Each evaluation invokes `c(left, right)` first. If it returns nonzero, that sign is returned and no `others` entry is invoked. Otherwise entries are invoked in argument order with the same left and right values until the first nonzero result; remaining entries are skipped. The result is zero only when every reached comparator returns zero.

`Then` must make a shallow snapshot of the variadic comparator descriptor slice during construction. Replacing an entry in the caller's source slice later must not change the existing composition. Function values and state captured by them remain shallow aliases, so later captured-state mutation remains visible. A non-empty snapshot may allocate at construction; evaluation must not allocate because of the composition itself.

#### `ThenBy`

```go
func (c Func[T]) ThenBy[K cmp.Ordered](key func(T) K) Func[T]
```

Appends one ascending ordered-key level. Each evaluation invokes `c(left, right)` first and returns its nonzero result unchanged. Only after a zero result does it invoke `key(left)` exactly once, invoke `key(right)` exactly once, and return `cmp.Compare(leftKey, rightKey)`. A receiver panic or nonzero result skips both projections. Projection panic and nil behavior otherwise match `By`.

#### `ThenByDescending`

```go
func (c Func[T]) ThenByDescending[K cmp.Ordered](key func(T) K) Func[T]
```

Has the same receiver-first short-circuit, projection order, count, panic, nil, and caching contract as `ThenBy`. After reaching and computing left and right keys, it returns `cmp.Compare(rightKey, leftKey)`. It reverses only the newly appended level and never changes the preceding ordering represented by `c`.

#### `ThenOn`

```go
func (c Func[T]) ThenOn[K any](project func(T) K, compare Func[K]) Func[T]
```

Appends one custom projected ordering level. Each evaluation invokes `c(left, right)` first and returns its nonzero result unchanged. Only after a zero result does it invoke `project(left)` exactly once, invoke `project(right)` exactly once, and invoke `compare(leftProjected, rightProjected)` exactly once. It returns the custom comparator result unchanged. Receiver or projection panic prevents every later call; a nil or panicking custom comparator is reached only after both projections return.

#### `ThenOnDescending`

```go
func (c Func[T]) ThenOnDescending[K any](project func(T) K, compare Func[K]) Func[T]
```

Has the same receiver-first short-circuit, projection order, comparator argument order, counts, panic, nil, and caching contract as `ThenOn`. When the custom level is reached, it maps the comparator result sign exactly as `OnDescending` does, including safe handling of `math.MinInt`. It reverses only the newly appended level and never changes the preceding ordering represented by `c`.

Every fluent projected-level method is construction-lazy and permits an independently inferred key type. Chaining ordered and custom levels of different types must compile without explicitly repeating receiver type `T`. These single-level methods capture their receiver and callbacks directly; they have no variadic descriptor slice to snapshot.

### 6.5 Complexity, interoperability, and consumer boundaries

| API | Construction | Evaluation time | Evaluation allocation | Short-circuit |
| --- | --- | --- | --- | --- |
| `Ordered` | `O(1)` | one `cmp.Compare` | zero | n/a |
| `By` | `O(1)` | two projections plus `cmp.Compare` | zero apart from callback behavior | left panic skips right |
| `ByDescending` | `O(1)` | two projections plus `cmp.Compare` | zero apart from callback behavior | left panic skips right |
| `On` | `O(1)` | two projections plus one comparator | zero apart from callbacks | projection panic skips later calls |
| `OnDescending` | `O(1)` | two projections, one comparator, sign reversal | zero apart from callbacks | projection panic skips later calls |
| `Reverse` | `O(1)` | one receiver call | zero | n/a |
| `Then` | `O(len(others))` snapshot | through first nonzero | zero | first nonzero |
| `ThenBy` | `O(1)` | receiver, then two projections plus `cmp.Compare` | zero apart from callbacks | nonzero receiver |
| `ThenByDescending` | `O(1)` | receiver, then two projections plus `cmp.Compare` | zero apart from callbacks | nonzero receiver |
| `ThenOn` | `O(1)` | receiver, then two projections plus one comparator | zero apart from callbacks | nonzero receiver |
| `ThenOnDescending` | `O(1)` | receiver, projected comparator, sign reversal | zero apart from callbacks | nonzero receiver |

The underlying type of `Func[T]` is identical to the unnamed `func(T, T) int` accepted by:

```text
slices.SortFunc
slices.SortStableFunc
slices.MinFunc
slices.MaxFunc
stream.Stream[T].SortedFunc
stream.Stream[T].MinFunc
stream.Stream[T].MaxFunc
```

A comparator value must pass directly to those APIs without conversion. Ordinary unnamed function values, instantiated generic functions such as `cmp.Compare`, and compatible method expressions such as `time.Time.Compare` must be accepted where a `Func` parameter supplies enough inference context. Generic fluent methods must support ordinary inference, explicit method instantiation, target-type inference for method values and method expressions, and chains whose projected levels use different key types.

Comparator specifies ordering only within one invocation. A consumer controls operand selection, total comparison count, stability, tie retention, collection mutation, and state after panic. `slices.SortStableFunc` and `Stream.SortedFunc` preserve source order inside equivalent groups; `slices.SortFunc` does not promise stability. Stable descending output reverses key-group direction but preserves tie order and therefore need not equal the complete reversal of stable ascending output.

## 7. Required examples

The implementation must provide executable Go examples for at least the following scenarios. Names may follow normal Go example naming, but behavior must match.

### 7.1 Optional chain

```go
name := optional.Some("  Brooklyn  ").
    Map(strings.TrimSpace).
    Filter(func(s string) bool { return s != "" }).
    Map(strings.ToUpper).
    OrElse("UNKNOWN")

fmt.Println(name)
// Output: BROOKLYN
```

### 7.2 Present nil and JSON

```go
presentNil := optional.Some[*int](nil)
data, _ := json.Marshal(presentNil)

var decoded optional.Optional[*int]
_ = json.Unmarshal(data, &decoded)

fmt.Println(presentNil.IsSome(), string(data), decoded.IsNone())
// Output: true null true
```

### 7.3 Infinite pipeline

```go
values := stream.Iterate(1, func(v int) int { return v + 1 }).
    Filter(func(v int) bool { return v%2 == 0 }).
    Map(func(v int) int { return v * v }).
    Take(5).
    Collect()

fmt.Println(values)
// Output: [4 16 36 64 100]
```

### 7.4 Pair and Seq2 interoperability

```go
seq2 := func(yield func(string, int) bool) {
    if !yield("a", 1) {
        return
    }
    yield("b", 2)
}

pairs := stream.FromSeq2(seq2).
    Map(func(p stream.Pair[string, int]) string {
        return fmt.Sprintf("%s=%d", p.First, p.Second)
    }).
    Collect()

fmt.Println(pairs)
// Output: [a=1 b=2]
```

### 7.5 Stable chunks, windows, and groups

Examples must demonstrate the final partial chunk, omission of partial windows, mutation safety after later output, and first-key GroupBy order.

### 7.6 Predicate composition and shared filters

Examples must demonstrate `Not`, `And`, and `Or`, projection with `On`, typed-nil handling, and one `Func[T]` passed directly to both Optional and Stream Filter.

### 7.7 Comparator mixed ordering and interoperability

Examples must demonstrate ascending and descending ordered keys through fluent `ThenBy` methods, a non-ordered key projected through `On` and fluent `ThenOn`, custom descending projection, the distinction between per-level descending and whole-order `Reverse`, and one `Func[T]` passed directly to both `slices.SortStableFunc` and Stream comparison operations.

## 8. Performance acceptance criteria

The implementation must include benchmarks against equivalent direct loops for sizes 10, 1K, and 1M where the scenario is meaningful:

```text
Filter + Map + Take
FlatMap
DistinctBy
Zip
Chunk
Window
SortedFunc
Collect
```

Every benchmark must report allocations and consume its result. Benchmarks must state whether source and destination storage are preallocated. A comparison is invalid if the direct loop and Shuttle version perform different copying, stable sorting, or ownership work.

Package comparator must include direct/composed, allocation-reporting evaluation benchmarks for:

```text
Ordered
By
ByDescending
On
OnDescending
Reverse
Then
ThenBy
ThenByDescending
ThenOn
ThenOnDescending
three-level mixed ordering
slices.SortStableFunc interoperability
Stream.SortedFunc interoperability
```

Comparator construction must be benchmarked separately from repeated evaluation. Sorting comparisons must copy the same unsorted input for every iteration on both sides and must not use unequal key-precomputation strategies.

Package predicate must include allocation-reporting benchmarks against equivalent direct Boolean expressions for:

```text
Not
And
Or
On
IsNil
Optional.Filter interoperability
Stream.Filter interoperability
```

They must report `ns/op`, `B/op`, and `allocs/op`. Timing is review data rather than a cross-platform pass/fail threshold.

Allocation tests must guard that, after bounded pipeline/traversal setup, allocation does not grow linearly with element count for:

```text
Comparator: Ordered, By, ByDescending, On, OnDescending, Reverse, Then,
            ThenBy, ThenByDescending, ThenOn, ThenOnDescending
Predicate: Not, And, Or, Always, Equal, EqualFunc, On
Optional: Map, FlatMap, Filter, Inspect
Stream:   Map, Filter, Inspect, Take, Skip, TakeWhile, SkipWhile
```

Comparator and Predicate allocation gates must preconstruct descriptors and require zero allocations per evaluation when callbacks do not allocate. Comparator construction closures and a non-empty `Then` snapshot may allocate before evaluation. `IsNil` and `IsNotNil` must also have representative allocation coverage for static nilable inputs, while documentation and benchmarks retain their interface-boxing and reflection cost boundary. Zero total allocations are a target where escape analysis permits for Optional and Stream, but their normative requirement is zero **per-element** allocation caused by the listed operators. `Ptr`, JSON, collection, maps, sorting buffers, chunk slices, and window slices are intentionally allocating operations.

## 9. Test acceptance criteria

### 9.1 Unit and property tests

The test suite must cover every public identifier and all documented empty, zero, nil, negative, duplicate, tie, overflow, and panic branches. It must include reusable and instrumented single-use sources and exact consumption counters.

Required Optional cases include:

```text
zero value; None; Some(zero); Some(typed nil)
Value and all fallback forms
Map; FlatMap; Filter; Inspect; Match; ZipWith; Flatten
Equal and EqualFunc callback selection
Ptr copy isolation
JSON null, malformed JSON, custom T marshalers, omitempty, omitzero
nil UnmarshalJSON receiver
unchanged receiver after JSON error
```

Required Comparator cases include:

```text
zero and nil Func; construction laziness
Ordered negative, zero, positive, named types, floating-point, NaN, and signed zero
By and ByDescending direction, exact left-right projection order, and counts
On and OnDescending projection/comparator order, arguments, exact counts, sign semantics, and every panic path
Reverse negative, zero, positive, math.MinInt, exact receiver count, and unchanged panic
double Reverse sign equivalence
Then left-to-right order, first-nonzero short-circuit, repeated evaluation, and all-zero result
ThenBy and ThenByDescending receiver-first short-circuit, direction, projection order, and counts
ThenOn and ThenOnDescending receiver-first short-circuit, comparator argument order, direction, and counts
custom descending math.MinInt safety without swapping comparator arguments
reached and skipped nil comparators
reached and skipped nil fluent projections and custom comparators
variadic descriptor snapshot and captured-state aliasing
generic inference, explicit instantiation, argument-context and target-type inference
generic fluent method values, method expressions, mixed key types, and named/unnamed function assignability
time.Time projection through On and time.Time.Compare
direct slices SortFunc, SortStableFunc, MinFunc, and MaxFunc interoperability
direct Stream SortedFunc, MinFunc, and MaxFunc interoperability
stable tie behavior and the distinction between whole Reverse and one descending level
safe concurrent immutable evaluation and caller-owned mutable state
zero per-comparison allocation for every core operation
```

Required Predicate cases include:

```text
zero and nil Func; construction laziness
Not, And, and Or truth tables
exact callback order, counts, short-circuiting, and repeated evaluation
reached and skipped nil predicates
unchanged panic propagation
variadic descriptor snapshots and captured-state aliasing
Always; Equal zero values; dynamic non-comparable Equal[any] panic
EqualFunc current/want order and exact count
On project/predicate order, exact count, and panic paths
nil interface and every nilable reflection kind
named nilable types; typed nils in interfaces; non-nil and non-nilable values
IsNotNil exact negation
generic inference, explicit instantiation, reverse inference
method values, method expressions, and named-function assignability
direct Optional.Filter and Stream.Filter interoperability
safe concurrent immutable evaluation and caller-owned mutable state
zero per-evaluation allocation for core composition
```

Required Stream cases include:

```text
empty; singleton; large; reusable; single-use; infinite
Map identity; Filter always true and false
all Take, Skip, TakeWhile, and SkipWhile boundaries
FlatMap with empty and infinite inner streams
stable DistinctBy order and per-traversal state
Chunk, Window, and every size/step relationship
slice retention and mutation after subsequent output
stable SortedFunc ties and Reverse
Zip left shorter, right shorter, and downstream early stop
Concat early stop before a later source starts
terminal empty behavior and short-circuit counts
panic propagation and iterator cleanup
dynamic non-comparable interface keys
integer boundary behavior for RangeStep, Enumerate, and Count where practical
```

Property tests and bounded fuzz targets must compare to straightforward reference expressions or loops. Comparator fuzzing must cover sign reversal, double-reversal sign equivalence, custom projected descending sign reversal, both `Then` and fluent mixed-key composition against direct lexicographic comparison, `By` and `ByDescending` against `cmp.Compare`, reversed ascending/descending key sequences while preserving stable ties, and strict-weak-order sign symmetry plus negative-relation and zero-equivalence transitivity for generated lawful combinations. It must not assert complete slice reversal when ties exist. Predicate fuzzing must cover double negation, both De Morgan laws, arbitrary Boolean `And` and `Or` truth tables, `IsNotNil(value) == !IsNil(value)`, and composed results against a direct reference expression. Stream fuzzing must never accidentally run an unbounded pipeline, and every fuzz target must bound retained input and work.

### 9.2 Race and leak tests

`go test -race ./...` must pass. Concurrency tests must distinguish:

- safe concurrent evaluation of immutable comparator compositions;
- safe concurrent evaluation of immutable predicate compositions;
- mutable state captured by comparators, which remains the caller's synchronization responsibility;
- mutable state captured by predicates, which remains the caller's synchronization responsibility;
- safe concurrent traversal of built-in immutable sources;
- intentionally unsafe external shared state, which is documented rather than normalized; and
- absence of races in per-traversal state when the same derived Stream is traversed concurrently over a concurrency-safe source and callbacks.

Iterator cleanup tests must use explicit completion signals or source defers. Timing-only goroutine-count assertions are insufficient. Zip tests must exercise normal completion, both shorter sides, downstream false, left panic, right panic, and panic in downstream processing, and must observe cleanup of both sources.

## 10. Release Definition of Done

Before `v1.0.0`, and before every later v1 release, all of the following must be true:

1. The repository builds with its declared Go 1.27 minimum.
2. During the RC phase the exact `go1.27rc3` commands pass; after stable, a pinned Go 1.27 stable patch replaces RC3.
3. `go test ./...`, `go test -race ./...`, and `go vet ./...` pass on the required native CI matrix.
4. All exported identifiers have complete GoDoc and every code example compiles.
5. Bounded fuzz runs and all committed fuzz seeds pass.
6. A pinned Go-1.27-capable `staticcheck` passes. RC tool lag must be explicitly recorded and cannot remain at v1 freeze.
7. `govulncheck ./...` passes for the release candidate.
8. Linux amd64/arm64, macOS amd64/arm64, and Windows amd64/arm64 compile; native tests run wherever reliable runners exist, with native versus cross-compiled coverage recorded.
9. Comparator and Predicate allocation targets and benchmarks, plus the Optional/Stream 10/1K/1M benchmark comparison, have been reviewed on a pinned runner.
10. An API diff against the previous release has been classified under Semantic Versioning.
11. The final Go 1.27 specification and release notes have been checked for generic-method or iterator changes.
12. `DESIGN.md`, this document, GoDoc, tests, and implementation agree on every public behavior.

## 11. Compatibility rules for implementers

Within v1, an implementation must not change:

- any declaration in the public inventories;
- zero-value behavior;
- comparator result-sign semantics, projection and custom-comparator argument order and counts, whole-order and per-level reversal, fluent receiver-first short-circuiting, variadic snapshots, and lack of key caching;
- predicate evaluation order, short-circuiting, variadic snapshots, and the strict typed-nil reflection boundary;
- None versus present-zero or present-nil semantics;
- JSON representations or omission behavior;
- empty collection nilness;
- callback selection or documented once-per-element guarantees;
- encounter, stable sort, tie, or group order;
- short-circuit and extra-consumption bounds;
- source replay inheritance;
- slice ownership and aliasing guarantees;
- panic conditions for invalid numeric inputs;
- lack of implicit concurrency, caching, or hidden errors; or
- the fields of `Pair` and `Group`.

The implementation may change internal representation, growth factors, hash-table sizing, sorting algorithm, comparator call pattern where unspecified, closure layout, diagnostic text, and non-normative performance constants.

Adding a method or exported field is an API change and requires review even if existing source appears to compile. Removing or changing API requires a new major version after v1, except for an unavoidable security or correctness emergency handled under the project's published policy.

## 12. Explicitly omitted APIs

The following names or concepts must not appear as v1 public API:

```text
Result, Either, Try, ErrorStream, MapErr
Stream2, KVStream, EntryStream
Parallel, Async, Observable
Collector or collectors framework
FromFile, FromReader, FromHTTP, FromChannel
MapToInt, MapToString
Peek, Limit, Select, Where, ToSlice, Fold
ContainsFunc (use Any)
Sum
Asc, Desc, OnAscending, ThenOnAscending, ByFunc, ThenByFunc
Compare, Comparing, ThenComparing, Chain, Natural, Default
comparator builders or direction enums
AllOf, AnyOf, Negate, Matches, Test, Apply, Compose, Contramap, Equals
reflection-based comparison, conversion, or equality outside predicate typed-nil detection
pre-Go-1.27 fallback functions for generic methods
```

Their omission is part of keeping v1 coherent; it does not reserve their future signatures.
