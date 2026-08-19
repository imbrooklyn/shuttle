# Shuttle

<!-- markdownlint-disable MD013 -->

Shuttle provides production-grade typed composition primitives for Go. Its v1
scope is intentionally narrow: composable `predicate.Func[T]` values, an eager
`Optional[T]`, and a lazy, ordered, sequential `Stream[T]`.

Shuttle requires Go 1.27 or newer because its fluent type-changing operations
use generic methods. Development before the stable Go 1.27 release is validated
with the pinned `go1.27rc3` toolchain.

## Packages

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
  terminals. It adds no hidden errors, worker goroutines, caching, or replay.

The complete rationale and behavioral contract are in [DESIGN.md](DESIGN.md)
and [API_SPEC.md](API_SPEC.md).

## Usage

```go
package main

import (
    "fmt"
    "strings"

    "github.com/imbrooklyn/shuttle/optional"
    "github.com/imbrooklyn/shuttle/predicate"
    "github.com/imbrooklyn/shuttle/stream"
)

func main() {
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

    fmt.Println(name)
    fmt.Println(values)
}
```

Output:

```text
BROOKLYN
[Brooklyn Shuttle]
```

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

- Optional and Stream have useful zero values; a zero Func is an ordinary nil
  function.
- Predicate composition snapshots variadic descriptors, adds no
  per-evaluation allocation of its own, and neither caches results nor starts
  goroutines.
- Optional callbacks run eagerly and only on the selected branch.
- Stream construction is lazy; incremental operators consume only on demand.
- Stream encounter order, early termination, and source cleanup are preserved.
- Chunk and window slices are independently owned with `cap(result) == len(result)`.
- Stateful traversal data is created separately for every traversal.
- Runtime packages use only the Go standard library.

Predicate values are safe for concurrent evaluation when their callbacks and
captured values are safe for concurrent use. Shuttle adds no locks around
caller-owned mutable state.

Reflection is used only by `predicate.IsNil` and, through it,
`predicate.IsNotNil`. This narrow exception is required to recognize a typed
nil stored in an interface. Other predicate operations, Optional, and Stream do
not use reflection for comparison, transformation, or composition.

Reusable and single-use behavior comes from the underlying iterator source.
Copying a Stream copies only its descriptor and never caches, forks, or rewinds
the source.

## Development

During the Go 1.27 release-candidate phase, use the exact baseline toolchain:

```bash
go1.27rc3 test ./...
go1.27rc3 test -race ./...
go1.27rc3 vet ./...
```

All three packages include fuzz or property coverage and allocation-aware
benchmarks. CI distinguishes native tests and race tests from cross-compilation.

## Release strategy

Shuttle remains on `v0.x` while Go 1.27 is an RC. After Go 1.27 stable ships,
the project will pin a stable patch toolchain, rerun compiler probes and all
release gates, review RC-to-stable language changes, and publish `v1.0.0` only
if no breaking correction is required.

## License

Shuttle is licensed under the [MIT License](LICENSE).
