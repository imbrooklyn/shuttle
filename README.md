# Shuttle

<!-- markdownlint-disable MD013 -->

Shuttle provides production-grade typed composition primitives for Go. Its v1
scope is intentionally narrow: composable `comparator.Func[T]` and
`predicate.Func[T]` values, an eager `Optional[T]`, and a lazy, ordered,
sequential `Stream[T]`.

Shuttle requires Go 1.27 or newer because its fluent type-changing operations
use generic methods. The initial stable development and release-validation
baseline is Go 1.27.0.

> [!IMPORTANT]
> Shuttle has not published a stable v1 release. Pre-v1 versions are intended
> for evaluation and may change incompatibly after review.

## Packages

- [`comparator`](https://pkg.go.dev/github.com/imbrooklyn/shuttle/comparator)
  provides reusable three-way ordering descriptors with natural ordering,
  projection, per-level fluent direction, complete-order reversal, and
  lexicographic composition. A `comparator.Func[T]` is directly accepted by
  Shuttle and standard-library comparison APIs.
- [`predicate`](https://pkg.go.dev/github.com/imbrooklyn/shuttle/predicate)
  provides a small named function type, short-circuiting composition, equality
  and projection adapters, and typed-nil-aware predicates. A `predicate.Func[T]`
  is directly accepted by both Optional and Stream filters.
- [`optional`](https://pkg.go.dev/github.com/imbrooklyn/shuttle/optional)
  represents an absent or present value independently of the value's zero or
  nil state. Its zero value is `None`, operations are eager, and JSON integration
  uses the natural representation of the stored value.
- [`stream`](https://pkg.go.dev/github.com/imbrooklyn/shuttle/stream) wraps
  `iter.Seq` with lazy, ordered, sequential transformations and explicit
  terminals. `FlatMap` accepts arbitrary inner Streams, while `FlatMapSlice`
  directly expands slice-backed data. The package adds no hidden errors, worker
  goroutines, caching, or replay.

The complete rationale and behavioral contract are in [DESIGN.md](DESIGN.md)
and [API_SPEC.md](API_SPEC.md).

## Usage

```go
package main

import (
    "fmt"
    "strings"

    "github.com/imbrooklyn/shuttle/comparator"
    "github.com/imbrooklyn/shuttle/optional"
    "github.com/imbrooklyn/shuttle/predicate"
    "github.com/imbrooklyn/shuttle/stream"
)

func main() {
    type rankedName struct {
        Name  string
        Score int
    }

    nonBlank := predicate.Func[string](func(value string) bool {
        return strings.TrimSpace(value) != ""
    })

    name := optional.Some("  Brooklyn  ").
        Map(strings.TrimSpace).
        Filter(nonBlank).
        Map(strings.ToUpper).
        OrElse("UNKNOWN")

    values := stream.Of("Brooklyn", " ", "Shuttle").
        Filter(nonBlank).
        Collect()

    byScoreThenNameDescending := comparator.
        By(func(value rankedName) int { return value.Score }).
        ThenByDescending(func(value rankedName) string {
            return value.Name
        })
    ranked := stream.FromSlice([]rankedName{
        {Name: "Brooklyn", Score: 2},
        {Name: "Shuttle", Score: 1},
        {Name: "Optional", Score: 2},
    }).SortedFunc(byScoreThenNameDescending).Collect()

    fmt.Println(name)
    fmt.Println(values)
    fmt.Println(ranked)
}
```

Output:

```text
BROOKLYN
[Brooklyn Shuttle]
[{Shuttle 1} {Optional 2} {Brooklyn 2}]
```

For nested DTOs and other slice-backed trees, `FlatMapSlice` removes repeated
`FromSlice` adapters without weakening the general `FlatMap` API:

```go
names := stream.FromSlice(orders).
    FlatMapSlice(func(order AnimalOrder) []AnimalFamily { return order.Families }).
    FlatMapSlice(func(family AnimalFamily) []AnimalSpecies { return family.Species }).
    FlatMapSlice(func(species AnimalSpecies) []AnimalSubspecies { return species.Subspecies }).
    FlatMapSlice(func(subspecies AnimalSubspecies) []Animal { return subspecies.Animals }).
    Map(func(animal Animal) string { return animal.Name }).
    Collect()
```

The complete runnable example composes slice flattening, reusable predicates,
mixed ordering, grouping, Optional extrema, and chunking:

```bash
go run ./examples/animals
```

The complete comparator API is deliberately small:

```go
type Func[T any] func(left, right T) int

func Ordered[T cmp.Ordered]() Func[T]
func By[T any, K cmp.Ordered](key func(T) K) Func[T]
func ByDescending[T any, K cmp.Ordered](key func(T) K) Func[T]
func On[A, B any](project func(A) B, compare func(B, B) int) Func[A]
func OnDescending[A, B any](project func(A) B, compare func(B, B) int) Func[A]

func (c Func[T]) Reverse() Func[T]
func (c Func[T]) Then(others ...Func[T]) Func[T]
func (c Func[T]) ThenBy[K cmp.Ordered](key func(T) K) Func[T]
func (c Func[T]) ThenByDescending[K cmp.Ordered](key func(T) K) Func[T]
func (c Func[T]) ThenOn[K any](project func(T) K, compare func(K, K) int) Func[T]
func (c Func[T]) ThenOnDescending[K any](project func(T) K, compare func(K, K) int) Func[T]
```

Only a comparator result's sign is meaningful. `Then` evaluates ordering levels
from left to right and stops at the first nonzero result. `Reverse` reverses the
complete existing ordering without negating an arbitrary integer result, so it
is safe even when a comparator returns `math.MinInt`. `ThenBy`,
`ThenByDescending`, `ThenOn`, and `ThenOnDescending` append exactly one level
and evaluate it only when every preceding level is equivalent. Ordered and
custom projections run for left and then right exactly once per reached
comparison; custom comparators receive those projected values in left-right
order even for descending levels. Custom-comparator parameters are unnamed, so
ordinary functions, `Func` values, method expressions, and other compatible
named function types pass without conversion. Keys are not cached between
comparisons.

The complete predicate API is deliberately small:

```go
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

Composition is construction-lazy and evaluates synchronously from left to
right. `And` and `Or` short-circuit exactly like `&&` and `||`; reached nil
functions and callback panics retain ordinary Go behavior. A zero `Func[T]` is
a nil function, not an implicit true or false predicate.

`Some` never interprets a zero or nil payload as absence. An `Optional[*T]`
therefore has three possible states: absent, present with a nil pointer, and
present with a non-nil pointer. Use `Optional[*T]` only when that distinction is
meaningful:

```go
presentNil := optional.Some[*int](nil)
pointer := presentNil.Ptr()

fmt.Println(presentNil.IsSome()) // true
fmt.Println(pointer == nil)      // false
fmt.Println(*pointer == nil)     // true
```

Here `Ptr` returns a `**int` pointing to a shallow copy of the stored nil
pointer; it does not dereference the payload. Dereferencing `**pointer` would
still panic in the usual Go manner.

JSON encodes both `None[*T]()` and `Some[*T](nil)` as `null`; decoding `null`
produces `None`. This intentionally loses the presence bit for a present nil.

## Behavioral guarantees

- Optional and Stream have useful zero values; a zero comparator or predicate
  Func is an ordinary nil function.
- Comparator composition snapshots variadic descriptors, adds no
  per-comparison allocation of its own, and neither caches keys nor starts
  goroutines.
- Predicate composition snapshots variadic descriptors, adds no
  per-evaluation allocation of its own, and neither caches results nor starts
  goroutines.
- Optional callbacks run eagerly and only on the selected branch.
- Stream construction is lazy; incremental operators consume only on demand.
- Stream encounter order, early termination, and source cleanup are preserved.
- Chunk and window slices are independently owned with `cap(result) == len(result)`.
- Chunk and window working buffers allocate lazily and do not select an initial
  capacity proportional to an untrusted requested size.
- Stateful traversal data is created separately for every traversal.
- Runtime packages use only the Go standard library.

Comparator and predicate values are safe for concurrent evaluation when their
callbacks and captured values are safe for concurrent use. Shuttle adds no
locks around caller-owned mutable state.

Reflection is used only by `predicate.IsNil` and, through it,
`predicate.IsNotNil`. This narrow exception is required to recognize a typed
nil stored in an interface. Comparator, other predicate operations, Optional,
and Stream do not use reflection for comparison, transformation, or
composition.

Reusable and single-use behavior comes from the underlying iterator source.
Copying a Stream copies only its descriptor and never caches, forks, or rewinds
the source.

## Development

Use the pinned stable baseline toolchain for development and release validation:

```bash
go version
go test ./...
go test -race ./...
go vet ./...
```

For the initial stable baseline, `go version` must report Go 1.27.0. A future
Go 1.27 patch upgrade must update the documented and CI toolchain pins together.

All four packages include fuzz or property coverage and allocation-aware
benchmarks. CI distinguishes native tests and race tests from cross-compilation.
Release benchmark comparison and regression classification are documented in
[BENCHMARKS.md](BENCHMARKS.md). The reviewed public API and GoDoc snapshot,
including its regeneration command, is documented in [api/README.md](api/README.md).
Release owners must follow [RELEASING.md](RELEASING.md). User-visible changes
are recorded in [CHANGELOG.md](CHANGELOG.md), and private vulnerability reports
follow [SECURITY.md](SECURITY.md).

## Release strategy

Go 1.27.0 is the stable v1 toolchain baseline. Shuttle remains on `v0.x` until
the stable-toolchain compiler probes, release gates, and final API review are
complete. The project will publish `v1.0.0` only if no breaking correction is
required.

## License

Shuttle is licensed under the [MIT License](LICENSE).
