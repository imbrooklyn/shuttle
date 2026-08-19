# Shuttle Design

<!-- markdownlint-disable MD013 MD024 -->

Status: authoritative design for Shuttle v1  
Module: `github.com/imbrooklyn/shuttle`  
Minimum language version: Go 1.27

This document explains why Shuttle is designed as specified. `API_SPEC.md` is the normative implementation contract when exact signatures or edge-case behavior are required. If the documents appear to conflict, `API_SPEC.md` takes precedence for implementation details; the conflict must nevertheless be fixed before release.

## 1. Purpose

Shuttle provides production-grade typed composition primitives for Go. Version 1 deliberately contains only three packages:

```text
github.com/imbrooklyn/shuttle/predicate
github.com/imbrooklyn/shuttle/optional
github.com/imbrooklyn/shuttle/stream
```

The governing priority is:

```text
correctness
> predictability
> API clarity
> Go idiomaticity
> performance
> convenience
> API quantity
```

Shuttle is not a port of Java Stream. Its model is Go's `iter.Seq`, Go 1.27 generic methods, concrete value types, synchronous callbacks, and explicit terminal operations. The library should make ordinary control flow easier to compose without hiding concurrency, errors, ownership, or resource lifetime.

“Narrow but deep” is a release constraint. An operator belongs in v1 only when it is common, has a clear canonical name, cannot be replaced by a trivial and equally readable composition, and has semantics Shuttle can maintain for the lifetime of v1.

## 2. Scope and non-goals

The v1 scope is `predicate.Func[T]` and its focused constructors and adapters, `Optional[T]`, `Stream[T]`, their standard-library iterator interoperability, and `encoding/json` support for `Optional[T]`. Predicate values are designed to pass directly to both Optional and Stream filters without either package importing `predicate`.

The following are explicitly outside v1:

- `Result`, `Either`, validation, or a hidden stream error channel;
- pipelines as a separate abstraction;
- parallel streams, worker pools, async tasks, observables, or reactive-streams protocols;
- a collectors framework;
- reflection-based transformations or conversion through `any`, apart from the narrowly specified typed-nil detection in `predicate.IsNil` and `predicate.IsNotNil`;
- file, reader, HTTP, or channel sources;
- I/O ownership or close semantics;
- a root `shuttle` package;
- compatibility shims for Go 1.26 or earlier;
- aliases such as `Peek`, `Limit`, `Select`, `Where`, `ToSlice`, or type-specific operations such as `MapToInt`.

Errorful transformations can use an element type chosen by the caller, including a future `Stream[Result[T]]`; the core `Stream` never stores a latent error. I/O sources require cancellation, blocking, and ownership contracts that do not belong in the core sequence abstraction. Channel conversion is similarly omitted because a channel does not by itself define cancellation when downstream stops.

## 3. Go 1.27 and release staging

Shuttle uses Go 1.27 generic methods in its first public implementation. It does not publish package-level fallbacks merely for older compilers, use build tags for a pre-1.27 API, or constrain the design to syntax accepted by Go 1.26. Package functions required by receiver-element constraints are part of the native Go 1.27 design. The separate package functions for structurally transformed results reflect a confirmed Go 1.27 compiler limitation discussed in Section 13, not pre-1.27 compatibility shims.

The pre-stable development baseline is the fixed `go1.27rc3` toolchain. The module's `go` directive is `1.27`, and CI must invoke the pinned RC rather than an unpinned `tip` build. Generic method declarations, inference, method expressions, receiver rules, and iterator behavior must be checked with that compiler, not inferred only from proposals.

The expected local setup and primary RC commands are:

```bash
go install golang.org/dl/go1.27rc3@latest
go1.27rc3 download
go1.27rc3 test ./...
go1.27rc3 test -race ./...
go1.27rc3 vet ./...
```

Until Go 1.27 stable is released, Shuttle remains at `v0.x`. RC-era releases may contain the complete implementation and may change incompatibly after review. They must not be tagged `v1.0.0`.

After Go 1.27 stable is released, the release owner must:

1. replace the RC toolchain in CI with a pinned Go 1.27 stable patch release;
2. run the complete test, race, fuzz, vet, static-analysis, vulnerability, and benchmark suites;
3. rerun compiler probes for generic methods, inference, receiver constraints, method values, and iterator behavior;
4. review the final Go 1.27 specification and release notes for RC-to-stable changes;
5. perform a final source and behavioral compatibility review against this specification; and
6. tag `v1.0.0` only if no breaking correction is needed.

An RC compiler bug must be investigated against the current official specification and issue tracker. Shuttle must not permanently encode an RC-only workaround into its public API.

## 4. Repository and dependency structure

The repository is one Go module:

```text
shuttle/
├── go.mod
├── README.md
├── DESIGN.md
├── API_SPEC.md
├── LICENSE
├── predicate/
├── optional/
└── stream/
```

There is no package at the module root. A root package would add an import and naming decision without owning a coherent abstraction.

The package dependency direction is intentionally acyclic:

```text
predicate ──────────────────────▶ Go standard library
stream    ───────▶ optional ────▶ Go standard library
  └────────────────────────────▶ Go standard library
```

`stream` imports `optional` for `FilterMap` and optional-returning terminals. `optional` never imports `stream`. `Pair` and `Group` live in `stream`, where paired iteration and ordered grouping need them. Optional composition therefore uses a combining callback rather than importing a stream tuple type.

`predicate` imports neither `optional` nor `stream`; its named function type is assignable to their existing unnamed `func(T) bool` callback parameters. Neither Optional nor Stream imports `predicate`, so predicate interoperability adds no package cycle and does not change their public APIs. Predicate otherwise uses only the standard library, with `reflect` confined to typed-nil detection.

Runtime packages have no third-party dependencies. Pinned development tools such as `staticcheck`, `govulncheck`, and an API-diff tool are build and release tooling, not runtime dependencies.

## 5. Concrete value types

The central abstractions are concrete value types:

```go
type Optional[T any] struct { /* unexported */ }
type Stream[T any] struct { /* unexported */ }
```

Concrete types are intentional. Go interfaces cannot declare generic methods in Go 1.27, and a concrete generic method does not implement a non-generic interface method merely because one instantiation looks compatible. Defining an abstraction interface would therefore remove the fluent type-changing methods or produce a second, weaker API.

Both types are cheap value handles. Copying an `Optional` copies its stored value shallowly. Copying a `Stream` copies a sequence function value; it does not copy, cache, fork, or reset the source behind that function.

Fields remain unexported so callers cannot construct invalid state or depend on representation. The zero values are useful by contract:

```go
var o optional.Optional[int] // None
var s stream.Stream[int]     // empty Stream
```

Zero-value usability removes mandatory constructors from ordinary structs and makes failed or omitted initialization safe. It does not imply that a copied single-use stream becomes reusable.

A nil pointer to either value type is not an additional state. Except for the deliberately specified nil receiver behavior of `(*Optional[T]).UnmarshalJSON`, calling a value-receiver method through a nil pointer has ordinary Go nil-pointer dereference behavior.

## 6. Optional model

### 6.1 Presence is independent of the value

`Optional[T]` has exactly two logical states: absent, or present with one `T`. Presence is represented separately from the zero value of `T`.

Consequently, all of these are present values:

```go
optional.Some(0)
optional.Some("")
optional.Some(false)
optional.Some[[]int](nil)
optional.Some[*User](nil)
optional.Some[any](nil)
```

The last three are not `None`. An untyped `nil` does not provide enough information to infer `T`, so callers must supply a typed nil or an explicit type argument. This is a language inference rule, not an Optional exception.

Values are copied with normal Go assignment semantics. A slice, map, pointer, interface, or function inside `Some` is not deep-copied. Mutating referenced data has the same aliasing consequences as copying that Go value directly.

When `T` is comparable, `Optional[T]` is comparable. Two absent values compare equal; two present values compare according to `T`'s `==`; and absent never equals present. The usual possibility of a run-time panic when comparing interface values containing dynamically non-comparable values still applies.

### 6.2 Optional operations are eager

Optional transformations are ordinary method calls, not deferred computations. `Map`, `FlatMap`, `Filter`, and `Inspect` invoke their callback during the call if, and only if, the receiver is present. `Match` invokes exactly the selected branch. Supplier forms such as `OrElseGet` and `OrGet` exist where avoiding eager fallback evaluation materially matters.

No operation catches panics. A callback panic propagates through the call. A nil callback only panics if execution reaches the callback; an absent branch does not invoke it.

`Ptr` returns a pointer to a new shallow copy. It never exposes the Optional's internal storage, and modifying the pointed-to value does not change the Optional.

### 6.3 Composition shape

Type-preserving operations are methods. Type-changing operations use Go 1.27 generic methods when the result element is an independent method type parameter. Symmetric constrained comparison remains package-level:

- `Map`, `FlatMap`, `Match`, and `ZipWith` need method-local result types;
- `Or` and `OrGet` preserve `T` and remain fluent methods;
- `Flatten` is a package function because only `Optional[Optional[T]]` is valid;
- `Equal` and `EqualFunc` are package functions because equality is symmetric and `Equal` requires `T comparable`.

The name `ZipWith` is deliberate: it combines two present values with a function. Calling it `Zip` would imply a shared tuple result, which would either put an unrelated `Pair` in `optional` or create a package cycle. Shuttle provides one unambiguous name and no alias.

### 6.4 JSON is necessarily lossy for present nils

`Optional[T]` integrates with the Go 1 `encoding/json` package:

- `None` marshals as JSON `null`;
- `Some(v)` marshals exactly as `v`;
- JSON `null` unmarshals as `None`;
- any other successfully decoded JSON value becomes `Some(v)`;
- `IsZero` reports true exactly for `None`.

This mapping cannot round-trip the presence bit for a present nil. `Some[*T](nil)`, `Some[[]T](nil)`, and `Some[any](nil)` all encode as `null` and decode as `None`. Shuttle does not add a tagged wrapper object merely to preserve that distinction, because doing so would make Optional values incompatible with the natural JSON representation of `T`.

Under `encoding/json`, `omitzero` uses `IsZero`: it omits `None` but retains `Some(zero)` and `Some(nil)`. The legacy `omitempty` definition does not consider a struct-valued Optional empty; a `None` field tagged only `omitempty` is therefore present as `null`. These field-tag rules are part of the v1 JSON contract.

Optional is not a general nullable-field framework. In particular, it does not attempt to distinguish a missing JSON object member from an explicit `null` when decoding a whole zero-valued struct: both leave the field as `None`.

## 7. Stream model

### 7.1 Semantic representation

`Stream[T]` is semantically a thin wrapper around `iter.Seq[T]`:

```go
type Stream[T any] struct {
    seq iter.Seq[T]
}
```

The actual field is unexported and may differ, but the behavioral model is fixed. A nil internal sequence means empty. `Seq` always returns a non-nil valid sequence, including for the zero Stream.

A Stream is:

- lazy at pipeline construction;
- ordered;
- sequential;
- non-caching;
- synchronous from Shuttle's perspective; and
- free of implicit worker goroutines.

It is not a collection, promise, subscription, or replay buffer. It has no stored length and no hidden terminal result.

### 7.2 Encounter order

Encounter order is the order in which the source calls `yield`. Unless an operator explicitly changes it, every operator preserves that order. `DistinctBy` preserves the first occurrence of each key. `GroupBy` orders groups by first key occurrence and values by source order. `SortedFunc` performs a stable sort. `Reverse` reverses the complete finite encounter order.

Map iteration order remains whatever order the source iterator provides. Shuttle never sorts or otherwise stabilizes an unordered source implicitly.

### 7.3 Traversal, reuse, and single use

Calling a terminal operation or invoking the sequence returned by `Seq` starts one traversal. Pipeline construction and `Seq()` itself do not consume upstream.

Reusable and single-use behavior is inherited from the source `iter.Seq`:

- built-in value sources such as `Empty`, `Of`, `FromSlice`, `Range`, `RepeatN`, and `Repeat` can be invoked repeatedly;
- `FromSeq` and `FromSeq2` preserve the supplied iterator's documented behavior;
- a derived Stream is no more reusable than its source;
- operator state such as a distinct-key map, chunk buffer, or scan accumulator is created afresh for each traversal;
- copying a Stream that wraps a single-use source creates another handle to the same source, not an independent traversal.

After early termination, a later traversal of a single-use source may continue, restart, or yield nothing exactly as that source specifies. Shuttle does not add a consumed flag or cache values to normalize these possibilities.

`Generate` shares the supplied function across traversals. A stateful generator therefore continues from its captured external state unless the function itself resets. `Iterate` creates a fresh current value from its seed per traversal, but a stateful `next` callback can still make results traversal-dependent.

### 7.4 Early termination and cleanup

Every Shuttle sequence obeys the `iter.Seq` protocol. It stops calling `yield` immediately after `yield` returns false and propagates false upstream. This property is required for short-circuiting and resource cleanup.

Sources own their resources and must arrange cleanup before their iterator function returns. When a terminal such as `First`, `Find`, `Any`, or `Take` has enough values, Shuttle returns false upstream so a conforming source can run deferred cleanup.

`Zip` requires pull-style coordination for its right side while its left side remains the push-style driver. Its implementation creates the right `iter.Pull` lazily only after obtaining a left value, and arranges `stop` on every exit path, including downstream early termination and panic unwinding. `next` and `stop` are never called concurrently. False is returned directly to the left source so it can clean up. Shuttle itself starts no goroutine. These rules avoid starting an unused right source and prevent the pull iterator from being stranded when either side ends or downstream stops.

Shuttle cannot repair a non-conforming source that ignores false, calls `yield` again after false, leaks its own goroutines, or fails to release its own resources.

### 7.5 Panic behavior

Shuttle does not recover panics from sources, callbacks, comparators, downstream yield functions, or the Go runtime. Deferred iterator cleanup still runs according to ordinary Go panic semantics. Numeric argument errors documented by the API panic immediately when the operator is constructed; they are programming errors, not stream elements or hidden errors.

The conditions under which Shuttle itself panics are compatibility commitments. Exact panic text is diagnostic and is not a v1 compatibility guarantee.

## 8. Laziness and demand

All intermediate Stream operators are lazy at construction. They fall into two execution classes.

Incremental operators request upstream values only as demand progresses:

```text
Map, FlatMap, Filter, FilterMap, Inspect
Take, Skip, TakeWhile, SkipWhile, Scan
Enumerate, Concat, Zip, DistinctBy
Chunk, Window, WindowStep
```

Barrier operators defer work until traversal, but must consume the entire upstream before their first output:

```text
SortedFunc, Reverse
```

Terminal operations begin consumption immediately. `Collect`, `AppendTo`, `Count`, `Last`, reductions, extrema, map collection, and grouping require end-of-stream. Other terminals stop as soon as their answer is known.

For example:

```go
source.
    Map(f).
    Filter(p).
    Take(3).
    Collect()
```

consumes only enough source elements to produce three post-filter values. `Map` runs once for each source element examined, `Filter` runs once for each mapped value examined, and neither callback runs for later elements.

Some incremental operators necessarily examine an element that they do not emit:

- `TakeWhile` consumes and tests the first failing element;
- `SkipWhile` consumes and tests skipped elements and emits the first failing element;
- `Filter` and `FilterMap` consume rejected elements;
- `Zip` pulls its left stream first, so if the right stream ends it may consume one unmatched left element. If the left stream ends first, it does not pull an unmatched element from the right stream.

These details matter for sources with side effects and are therefore specified rather than treated as implementation accidents.

`Take(0)` is stronger than merely producing no values: it does not invoke upstream at all. `Take(n)` stops upstream immediately after accepting its nth value and does not probe for an `(n+1)`th value.

## 9. Infinite streams

Infinite streams are first-class. The API uses three documentation categories:

- **infinite-safe**: can process an infinite input incrementally with bounded operator state, although a predicate may naturally delay output forever;
- **conditionally infinite-safe**: can be useful on infinite input but termination or bounded memory depends on data or an early answer;
- **finite-only**: requires end-of-stream to emit its first value or return its terminal result.

`Map`, `Filter`, `Take`, `Skip`, `Scan`, `Zip`, `Chunk`, and `Window` are infinite-safe. `DistinctBy` is conditional because its key set can grow without bound and an infinite suffix containing only seen keys can consume forever without output. `Any`, `All`, `None`, `Find`, and `ForEachErr` are conditional because they return on a decisive value or error but otherwise await the end. `SortedFunc`, `Reverse`, `Collect`, `Count`, `Last`, reductions, extrema, `ToMap`, and `GroupBy` are finite-only.

“Finite-only” is a precondition on useful completion, not a run-time check. Shuttle cannot detect that a source is infinite. Applying a barrier or full terminal to an infinite stream can run forever or exhaust memory.

## 10. Concurrency model

Core Stream has no implicit concurrency. `Map`, `Filter`, `FlatMap`, and every other operator invoke callbacks sequentially within one traversal and never create a worker pool. Shuttle never invokes a user callback concurrently with itself as part of one conforming traversal.

Shuttle requires a source to call its `yield` function serially and to wait for each call to return. A source that calls `yield` concurrently is outside the supported iterator contract and can race or panic. Shuttle does not promise goroutine identity for callbacks supplied to a Stream: an external source controls the context in which it calls `yield`, and `iter.Pull` uses the standard runtime's coroutine mechanism.

A Stream value is immutable after construction, but “immutable handle” is not the same as “safe for concurrent consumption.” Concurrent traversals are safe only when all of the following are safe concurrently:

- the source sequence;
- captured source state and referenced data, such as a slice backing array;
- every callback and comparator; and
- any destination passed to a terminal operation.

Operator-local state is per traversal and introduces no cross-traversal race. Shuttle does not serialize access to a single-use source or to shared callbacks. In particular, concurrent traversal of the same `Generate` source or an externally stateful `iter.Seq` is the caller's responsibility.

The functions returned by `iter.Pull` are used only within the traversal that created them. They are not exposed by Shuttle and are never called simultaneously from multiple goroutines.

## 11. Memory and ownership model

All value movement is shallow Go assignment unless a slice-output rule says otherwise.

`Of` makes a shallow snapshot of its variadic argument slice at construction. This prevents later replacement of elements in a slice passed as `values...` from changing the stream. Referenced objects inside elements remain shared.

`FromSlice` is the explicit zero-copy view. It captures the supplied slice header, including its length, and reads elements during each traversal. Replacing an element in the captured range is visible to later traversal; changing the caller's slice length is not. Mutation concurrent with traversal follows normal Go data-race rules.

`Chunk`, `Window`, and `WindowStep` prioritize ownership safety over buffer reuse. Every emitted slice has stable contents, its own backing storage distinct from every other emitted slice and from internal working buffers, and `cap(result) == len(result)`. Callers may retain or mutate it after the next value is requested. Element-internal references remain shallow aliases.

`Collect` returns a newly built slice and returns nil for an empty stream, matching `slices.Collect`. `AppendTo` follows `append` ownership: it may reuse the destination backing array and preserves the destination's nilness when no values are appended. `ToMap` and `ToMapWith` always return a newly allocated non-nil map. `GroupBy` returns nil for empty input and returns stable group slices for non-empty input.

Stateful operators allocate state per traversal:

- `DistinctBy`: a map proportional to distinct keys seen;
- `Chunk`: up to one chunk plus each emitted owned slice;
- `Window`: an `O(size)` working buffer plus one owned slice per emitted window;
- `SortedFunc` and `Reverse`: all upstream values;
- terminal maps and grouping: all retained output.

No intermediate operator caches a complete traversal for reuse.

## 12. Error model

`Stream[T]` carries only `T`. It has no hidden error field and no method changes behavior based on an unobserved prior error.

`ForEachErr(func(T) error) error` is the sole error-aware boundary in v1. It is a terminal control-flow helper: callbacks run in encounter order; the first non-nil error stops upstream and is returned unchanged. This does not turn Stream into an error stream and does not support errorful lazy transformation.

Construction and transformation functions return no errors because their documented failures are programming errors (invalid sizes or steps), callback panics, or deferred source behavior. Error-returning I/O constructors are out of scope.

## 13. Generic methods and constrained operations

Go 1.27 lets a concrete method declare new type parameters, enabling fluent type-changing methods such as:

```go
func (s Stream[T]) Map[R any](fn func(T) R) Stream[R]
func (s Stream[T]) GroupBy[K comparable](key func(T) K) []Group[K, T]
```

It does not let a method strengthen the receiver's existing `T any` to `T comparable` or `T cmp.Ordered`. Constraining `Stream[T]` itself would reject valid streams of slices, maps, and functions merely to support a few conveniences.

The Go 1.27 RC3 type checker also rejects a method when its result recursively instantiates the receiver generic type with a structural expansion of the receiver parameter. For example, these declarations produce an `instantiation cycle` in `go1.27rc3`:

```go
func (s Stream[T]) Enumerate() Stream[Pair[int, T]]
func (s Stream[T]) Chunk(size int) Stream[[]T]
func (s Stream[T]) Zip[U any](other Stream[U]) Stream[Pair[T, U]]
```

The equivalent generic package functions compile. This behavior is tracked by Go issue [#80172](https://go.dev/issue/80172) as an overly conservative method monomorphization check and is targeted after Go 1.27. Shuttle cannot declare an API that its minimum supported stable toolchain rejects, and it must not hide the problem behind `any`, reflection, or a compiler-specific type-inference trick.

This is explicitly rechecked at the RC-to-stable gate. If the final Go 1.27 compiler accepts the direct method declarations, maintainers must revisit these package-function choices before v1 freeze and update both specifications; they must not add duplicate method aliases silently. If stable retains the rejection, the package functions are the supported Go 1.27 API rather than an RC-only workaround.

The design therefore separates operations as follows:

- methods accepting a key or comparison function work for all `T`: `DistinctBy`, `SortedFunc`, `MinFunc`, `MaxFunc`, `MinBy`, and `MaxBy`;
- common operations whose element itself must satisfy a constraint are package functions: `Distinct`, `Contains`, `Sorted`, `Min`, and `Max`;
- transformations whose result structurally embeds the receiver element are package functions: `Enumerate`, `Zip`, `Chunk`, `Window`, and `WindowStep`;
- `Sum` is omitted. It would require a new public numeric constraint and policy for integer overflow, floating-point order, complex values, durations, and user-defined numeric types. `Reduce` is explicit and sufficient in v1.

These package functions are not pre-1.27 compatibility fallbacks. They are the valid Go 1.27 expression of receiver-element constraints and structurally transformed receiver types. Generic methods remain the canonical form for `Map`, `FlatMap`, `FilterMap`, `Scan`, and other results based on an independently inferred method type parameter.

## 14. Pairs, Seq2, and deterministic grouping

Shuttle keeps a single `Stream[T]` abstraction. It does not duplicate every operator into `Stream2` or `KVStream`.

```go
type Pair[A, B any] struct {
    First  A
    Second B
}
```

`FromSeq2` converts each pair to `Pair`, `ToSeq2` performs the inverse, `Enumerate` uses `Pair[int, T]`, and `Zip` uses `Pair[T, U]`. The fields are exported to make construction and pattern-free access straightforward.

Grouping returns an ordered slice:

```go
type Group[K comparable, V any] struct {
    Key    K
    Values []V
}
```

A `map[K][]V` would preserve values within a bucket but would discard first-key order when ranged. `[]Group[K,V]` gives deterministic behavior without a second ordered-map abstraction. A private map from key to group index provides expected `O(n)` grouping time.

`Pair` and `Group` are intentionally small frozen data records. Adding fields to either after v1 would break unkeyed literals and can change comparability; their field sets are part of the compatibility contract.

## 15. API selection and rejected aliases

Each public name represents one concept. The following choices are deliberate:

- `Inspect`, not `Peek`;
- `Take`, not `Limit`;
- `Filter`, not `Select` or `Where`;
- `Collect`, not `ToSlice`;
- `Reduce`, not a duplicate `Fold`;
- `Any`, `All`, and `None`, with no `AnyMatch` aliases;
- `ZipWith` for Optional combining and `Zip` for paired streams;
- `SortedFunc`, using a stable comparison sort, with no second stable-sort name;
- `WindowStep(s, size, step)`, with `Window(s, size)` as the common step-one form;
- `DistinctBy` plus the constrained `Distinct` function, with no reflection-based equality;
- `Contains` only for comparable elements; arbitrary matching is already `Any(predicate)`.

`FromFile`, `FromReader`, `FromHTTP`, and `FromChannel` are excluded because early termination needs an ownership or cancellation protocol. `Parallel` is excluded because ordering, error arbitration, backpressure, and goroutine lifetime require a separate design.

No API is reserved merely for symmetry. New v1 surface requires a concrete use case and a compatibility review.

## 16. Performance model

Shuttle does not promise to beat a hand-written loop. It does require that abstraction overhead remain understandable and that streaming operators avoid allocation proportional to element count when their semantics do not require it.

Targets:

- Predicate `Not`, `And`, `Or`, `Always`, `Equal`, `EqualFunc`, and `On` allocate zero times per evaluation after construction when callbacks themselves do not allocate; `And` and `Or` may allocate once to snapshot non-empty variadic descriptors at construction;
- Optional `Map`, `FlatMap`, `Filter`, `Inspect`, and extraction allocate zero times when callbacks and values do not escape;
- Stream `Map`, `Filter`, `Inspect`, `Take`, `Skip`, `TakeWhile`, and `SkipWhile` allocate zero times per element;
- pipeline construction and traversal setup may allocate a bounded number of closures or iterator frames;
- `Zip` may have bounded traversal setup cost from one `iter.Pull`, never per-pair allocation;
- stateful maps, barrier buffers, collected results, chunks, and windows allocate according to their documented output or working state.

Benchmarks compare a direct loop and Shuttle at input sizes 10, 1K, and 1M for:

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

Predicate benchmarks separately compare direct Boolean expressions with `Not`, `And`, `Or`, `On`, and `IsNil`, and include Optional and Stream Filter interoperability. They report `ns/op`, `B/op`, and `allocs/op`. `IsNil` benchmarks distinguish ordinary nilable values from typed nils stored in interfaces because reflection and interface boxing are its deliberate cost boundary.

They report `ns/op`, `B/op`, and `allocs/op`, use equivalent work and preallocation assumptions, consume results so the compiler cannot remove work, and separate pipeline construction from repeated traversal where relevant. Single-use and reusable sources receive separate benchmarks when setup materially differs.

Timing thresholds are not hard cross-platform CI gates because scheduler, compiler, and hardware variance makes them noisy. Allocation regressions in the zero-per-element operators are testable gates. Release candidates require a benchmark comparison on a pinned runner, with any material regression explained.

## 17. Test strategy

Unit tests are table-driven where boundaries vary and use explicit instrumented iterators where consumption counts or cleanup matter. Every exported operation has a normal example; central pipeline, Optional, JSON, infinite-stream, Seq2, chunk/window, and grouping examples are executable.

Optional tests cover zero value, `None`, present zero values, every nil-capable present value, eager callback counts, `Map`, `FlatMap`, `Filter`, `Inspect`, `Match`, `ZipWith`, fallback laziness, equality, pointer-copy isolation, JSON errors, field tags, and the lossy present-nil round trip.

Predicate tests cover a zero and nil `Func`, construction laziness, complete Boolean truth tables, exact left-to-right callback order and counts, short-circuiting around reached and skipped nil functions, unchanged panic propagation, variadic descriptor snapshots, normal aliasing of captured state, constructor and adapter argument order, every nilable reflection kind, named nilable types, typed nils stored in interfaces, generic inference, reverse inference, method values, method expressions, and direct Optional/Stream Filter use. Concurrent evaluation tests use immutable or explicitly synchronized captured state; unsynchronized caller-owned mutation is documented as a caller data race rather than executed in the race suite.

Stream tests cover empty, singleton, large, reusable, single-use, and infinite sources; identity and always-true/false properties; all count and window boundaries; empty inner streams; stable distinct and sorting behavior; shorter-left and shorter-right Zip behavior; early termination of Zip and Concat; exact callback and upstream-consumption counts; panic propagation; and cleanup after false or panic.

Property and fuzz tests compare operators with simple reference loops for finite generated slices. Important properties include:

- `Map(identity)` preserves values;
- `Filter(true)` preserves values and `Filter(false)` is empty;
- `Take(n).Concat(Skip(n))` reconstructs a finite reusable source for valid `n`;
- `Reverse().Reverse()` preserves finite values;
- flattening `stream.Chunk(s, k)` preserves the values of finite `s` for `k > 0`;
- `WindowStep` matches the mathematical window-start definition;
- `DistinctBy` emits exactly the first occurrence of each key;
- Seq2 round trips preserve ordered pairs;
- JSON never produces an invalid Optional state.

Predicate fuzz properties include double negation, both De Morgan laws, arbitrary Boolean `And` and `Or` truth tables, exact negation between `IsNil` and `IsNotNil`, and equivalence between composed predicates and direct Boolean reference expressions. Inputs and generated values remain deterministic and bounded.

Fuzz targets must be deterministic and bounded. Infinite-source tests always include a terminating operator or an instrumented forced stop. Leak-sensitive tests verify that every `iter.Pull` stop path runs and use the race detector; they do not rely only on timing-based goroutine counts.

Benchmarks, race tests, and examples complement rather than replace semantic assertions.

## 18. Release gates and CI

Every v1 release candidate must satisfy, with the pinned target toolchain:

```bash
go test ./...
go test -race ./...
go vet ./...
```

It must also satisfy:

- all exported identifiers have complete GoDoc beginning with the identifier name;
- all major operators document evaluation mode, order, complexity, short-circuiting, and infinite-stream status;
- examples compile and expected-output examples pass;
- fuzz seed corpora pass in ordinary tests;
- dedicated bounded fuzz runs pass on Linux amd64 and arm64-capable infrastructure;
- `staticcheck` passes with a pinned version that supports Go 1.27 syntax;
- `govulncheck ./...` passes against a recorded current database during release qualification;
- the public API diff contains only changes permitted by the intended semantic version;
- benchmark and allocation comparisons have been reviewed; and
- the English specification and local Chinese mirrors have matching headings, signatures, tables, and semantics.

The required platform matrix is:

```text
Linux:   amd64, arm64
macOS:   amd64, arm64
Windows: amd64, arm64 where a reliable native runner is available
```

At minimum, tests run natively on Linux amd64/arm64, macOS arm64, and Windows amd64; compilation is checked for every listed supported pair. Race tests run on every required native runner supported by Go's race detector. A platform without dependable public arm64 runners is not silently claimed as runtime-tested; the release record distinguishes native tests from cross-compilation.

`staticcheck` is required for v1 once its pinned release parses and analyzes Go 1.27 generic methods. During the RC window, tool lag may be a documented non-blocking job, but compiler, test, race, and vet jobs remain blocking. `govulncheck` is required on the default branch, on a schedule, and for release; network availability may make it retryable rather than a per-commit blocking job. Benchmark timing regression is review-gated on controlled hardware, not a noisy universal pass/fail threshold.

## 19. Compatibility policy

During `v0.x`, breaking changes are permitted but must be intentional, documented in release notes, and reflected in both specifications. Deprecation is preferred when practical but is not required before v1.

At v1, compatibility includes more than whether callers compile. The following are public contracts:

- exported names, type parameters, constraints, and signatures;
- zero-value and nil behavior;
- presence semantics and JSON mapping;
- encounter order, stable ordering, and tie behavior;
- laziness, barrier points, callback selection, and documented consumption bounds;
- short-circuiting and cleanup propagation;
- panic conditions, but not exact panic text;
- returned-slice ownership and aliasing;
- empty-result nilness for slices and allocation for maps;
- source replay inheritance and lack of caching;
- absence of implicit concurrency and hidden error state; and
- the fields of `Pair` and `Group`.

Performance numbers, comparator call order during sorting, capacity growth for collected slices and maps, concrete internal representation, and diagnostic text are not compatibility guarantees unless the API specification explicitly says otherwise.

After v1:

- patch releases fix bugs without adding API or intentionally changing specified behavior;
- minor releases may add backward-compatible API only after the same surface review used for v1;
- exported API is not removed or signature-changed within v1;
- a superseded API is documented as deprecated and retained for the remainder of v1 unless a security or correctness emergency makes that impossible;
- behavior changes that invalidate reasonable reliance on this specification require a new major version, even if source still compiles;
- fields are not added to `Pair` or `Group` within v1; and
- implementation changes must preserve Optional comparability for comparable `T`.

New operators are not automatically backward-compatible merely because they add methods: method additions can affect interface embedding, selector resolution, reflection, and user expectations. Every addition receives an API review.

## 20. Maintainer review conclusions

The v1 design has been checked specifically for API inflation, receiver-constraint errors, nil ambiguity, iterator lifetime, buffer aliasing, infinite-input traps, and compatibility hazards. The resulting high-value decisions are:

- keep predicate as an independent named-function package with exact short-circuit semantics, no aliases, and reflection confined to typed-nil detection;
- use generic concrete methods for independently typed fluent changes, but package functions for constraints on or structural expansions of `T`;
- keep `Optional` eager and `Stream` lazy;
- preserve present zero and present nil independently from absence, while documenting JSON's unavoidable loss;
- normalize a zero or nil-backed Stream to empty without normalizing source replay behavior;
- require false propagation and deferred `iter.Pull` stops;
- specify Zip's one-sided extra-consumption rule;
- make chunk and window outputs independently owned;
- use stable comparison sorting and deterministic ordered groups;
- expose one Pair-based stream rather than a parallel Seq2 API family;
- return nil empty slices where collection follows Go's `slices.Collect`, while returning non-nil terminal maps;
- omit `Sum`, I/O sources, concurrency, error streams, aliases, and collector abstractions; and
- treat behavioral semantics as versioned API, not merely implementation notes.

Any implementation that weakens one of these conclusions requires a specification change and a new review before merge.

## 21. Predicate composition model

### 21.1 Rationale and surface

Optional and Stream both accept the idiomatic unnamed callback type `func(T) bool`. Repeatedly writing anonymous functions is sufficient for one-off conditions but provides no focused vocabulary for reusing and composing a condition across both abstractions. Package `predicate` supplies that vocabulary without becoming a general utility package.

Its complete v1 surface is:

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

`AllOf`, `AnyOf`, `Negate`, `Matches`, `Test`, `Apply`, `Compose`, `Contramap`, `Equals`, and similar aliases are intentionally absent. Each supplied name represents one operation with one canonical spelling.

Go 1.27 RC3 compiler probes confirm that `Func[T]` is assignable to an unnamed `func(T) bool`, ordinary unnamed function values are accepted where `Func[T]` is required, constructors infer their type arguments, generic nil predicates support assignment and argument-context reverse inference, and `Func` methods support method values and method expressions. The candidate inventory therefore requires no compatibility workaround.

### 21.2 Composition semantics

A `Func[T]` is a named function value, not a struct or interface wrapper. Its zero value is nil. Shuttle does not reinterpret nil as an identity, constant true, or constant false predicate. Constructing `p.Not()`, `p.And(...)`, or `p.Or(...)` when `p` is nil does not evaluate it; evaluation panics by ordinary nil-function invocation if execution reaches it.

Composition is construction-lazy and evaluation is synchronous, serial, and left to right:

- `Not` invokes its receiver exactly once and negates the result;
- `And` invokes its receiver first and then each `others` entry until the first false result;
- `Or` invokes its receiver first and then each `others` entry until the first true result;
- a nil entry skipped by short-circuiting does not panic, while a reached nil entry does; and
- panics propagate unchanged without recovery.

`And` and `Or` make a shallow snapshot of the variadic predicate descriptor slice during construction. Replacing an entry in the caller's slice cannot change an existing composition. Function values and state captured by them retain ordinary shallow aliasing, so later mutation of captured data remains visible.

Evaluation creates no goroutine, acquires no global lock, caches no result, and allocates no memory after construction when callbacks themselves do not allocate. Non-empty variadic snapshots may allocate once at construction. A repeated evaluation always invokes the predicates required by that evaluation.

### 21.3 Constructors and adapters

`Always(result)` ignores its input. `Equal(want)` uses Go's `==`; it does not use reflection or deep equality. Consequently, an interface type satisfies `comparable`, but evaluating equality when a dynamic operand is non-comparable may panic in the ordinary Go manner.

`EqualFunc(want, equal)` invokes `equal(current, want)` exactly once per evaluation. The argument order is part of the API. A nil equality callback is accepted at construction and panics only if the returned predicate is evaluated.

`On(project, predicate)` implements a type-safe projection in the predicate-input direction. Each evaluation invokes `project` exactly once and, only after projection returns, invokes `predicate` exactly once with that result. A projection or predicate panic propagates unchanged, and projection is never recomputed.

### 21.4 Typed-nil reflection exception

`IsNil` and `IsNotNil` are generic functions rather than constructors, so matching instantiations can be passed directly as callbacks. `IsNil` returns true for a nil interface and for nil dynamic values of channel, function, map, pointer, unsafe-pointer, and slice kinds, including named types and typed nils stored in an interface. It returns false for non-nil values and non-nilable types without panicking. `IsNotNil(value)` is defined strictly as `!IsNil(value)`.

Distinguishing a typed nil hidden behind `T any` from a non-nil interface requires inspecting the dynamic kind. The implementation therefore uses `reflect.ValueOf`, `Value.Kind`, and `Value.IsNil` only inside `IsNil`; `IsNotNil` delegates to it. This is the sole reflection exception in predicate and does not permit reflective equality, projection, composition, Optional behavior, or Stream behavior. Values pass through the interface required by `reflect.ValueOf`, so interface boxing and reflection dispatch are explicit costs and may affect compiler escape decisions. No third-party runtime dependency is introduced.

### 21.5 Optional, Stream, and concurrency

Because the underlying type of `Func[T]` is exactly `func(T) bool`, one descriptor can be shared directly:

```go
nonBlank := predicate.Func[string](func(value string) bool {
    return strings.TrimSpace(value) != ""
})

maybeName := optional.Some("Brooklyn").Filter(nonBlank)
names := stream.Of("Brooklyn", " ", "Shuttle").Filter(nonBlank).Collect()
```

The packages do not import one another to make this work; it follows from Go assignability. The generic functions can likewise be used directly, such as `optional.Some(pointer).Filter(predicate.IsNotNil)` and `stream.FromSlice(pointers).Filter(predicate.IsNil)`.

An immutable composition is safe for concurrent evaluation when every callback, projected value, referenced object, and captured variable it reaches is safe for concurrent use. Shuttle adds no lock around caller state and never invokes predicates concurrently on its own. A caller that concurrently mutates unsynchronized captured state has the same data race it would have with a directly called closure.

## 22. Normative references

- [Go 1.27 release notes](https://go.dev/doc/go1.27)
- [Generic methods proposal accepted for Go 1.27](https://go.dev/issue/77273)
- [Go compiler method-instantiation issue #80172](https://go.dev/issue/80172)
- [Go language specification](https://go.dev/ref/spec)
- [`iter` package contract](https://pkg.go.dev/iter)
- [`encoding/json` package contract](https://pkg.go.dev/encoding/json)
- [Go module version numbering](https://go.dev/doc/modules/version-numbers)
- [Go 1 compatibility guidance](https://go.dev/doc/go1compat)
- [Go fuzzing guidance](https://go.dev/doc/security/fuzz/)
