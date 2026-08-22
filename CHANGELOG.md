# Changelog

All notable changes to Shuttle are recorded in this file. The project follows
[Semantic Versioning](https://semver.org/spec/v2.0.0.html); during `v0.x`,
breaking changes remain possible and must be called out explicitly.

## Unreleased

No changes yet.

## [0.1.0] - 2026-08-22

Initial public pre-v1 release.

### Added

- `comparator` primitives for natural, projected, reversed, and lexicographic
  three-way ordering.
- `predicate` primitives for short-circuiting composition, equality,
  projection, and typed-nil detection.
- Eager `optional.Optional[T]` values with zero-value, pointer, equality, and
  JSON integration.
- Lazy, ordered, sequential `stream.Stream[T]` sources, transformations,
  stateful operators, terminals, iterator adapters, and Optional integration.
- Executable package examples and a complete animals example.
- Multi-platform native tests, race tests, cross-compilation, bounded fuzzing,
  minute-scale release fuzz qualification, static analysis, vulnerability
  scanning, public API snapshots, and release benchmark comparison.

### Release notes

- Go 1.27.0 is the initial minimum and release-validation toolchain.
- Runtime packages use only the Go standard library.
- The public API remains unstable until `v1.0.0`.
- The release API review is recorded in
  [`docs/releases/v0.1.0-api-freeze.md`](docs/releases/v0.1.0-api-freeze.md).

[0.1.0]: https://github.com/imbrooklyn/shuttle/releases/tag/v0.1.0
