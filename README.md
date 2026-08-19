# Shuttle

<!-- markdownlint-disable MD013 -->

Shuttle provides production-grade typed composition primitives for Go. Its v1
scope is intentionally narrow: an eager `Optional[T]` and a lazy, ordered,
sequential `Stream[T]`.

Shuttle requires Go 1.27 or newer because its fluent type-changing operations
use generic methods. Development before the stable Go 1.27 release is validated
with the pinned `go1.27rc3` toolchain.

## Packages

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
    "github.com/imbrooklyn/shuttle/stream"
)

func main() {
    name := optional.Some("  Brooklyn  ").
        Map(strings.TrimSpace).
        Filter(func(value string) bool { return value != "" }).
        Map(strings.ToUpper).
        OrElse("UNKNOWN")

    values := stream.Iterate(1, func(value int) int { return value + 1 }).
        Filter(func(value int) bool { return value%2 == 0 }).
        Map(func(value int) int { return value * value }).
        Take(5).
        Collect()

    fmt.Println(name)
    fmt.Println(values)
}
```

Output:

```text
BROOKLYN
[4 16 36 64 100]
```

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

- Both public types have useful zero values.
- Optional callbacks run eagerly and only on the selected branch.
- Stream construction is lazy; incremental operators consume only on demand.
- Stream encounter order, early termination, and source cleanup are preserved.
- Chunk and window slices are independently owned with `cap(result) == len(result)`.
- Stateful traversal data is created separately for every traversal.
- Runtime packages use only the Go standard library.

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

Fuzz targets and allocation-aware direct-loop benchmarks are included in both
packages. CI distinguishes native tests and race tests from cross-compilation.

## Release strategy

Shuttle remains on `v0.x` while Go 1.27 is an RC. After Go 1.27 stable ships,
the project will pin a stable patch toolchain, rerun compiler probes and all
release gates, review RC-to-stable language changes, and publish `v1.0.0` only
if no breaking correction is required.

## License

Shuttle is licensed under the [MIT License](LICENSE).
