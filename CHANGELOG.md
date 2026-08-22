# Changelog

All notable changes to Shuttle are recorded in this file. The project follows
[Semantic Versioning](https://semver.org/spec/v2.0.0.html); during `v0.x`,
breaking changes remain possible and must be called out explicitly.

## Unreleased

This section is the draft release note for Shuttle's first public pre-v1
version.

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
  static analysis, vulnerability scanning, public API snapshots, and release
  benchmark comparison.

### Release notes

- Go 1.27.0 is the initial minimum and release-validation toolchain.
- Runtime packages use only the Go standard library.
- The public API remains unstable until `v1.0.0`.
