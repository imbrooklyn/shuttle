# Benchmark and regression policy

Shuttle separates deterministic allocation gates from review-oriented timing
data. The release benchmark workflow runs the baseline and candidate in the
same job with the pinned Go toolchain, ten samples, a 500 ms benchmark time,
`-p=1` package serialization, and `-benchmem`. It retains both raw outputs and
the pinned `benchstat` report.

## Blocking regressions

A release is blocked when any of the following is true:

- an allocation unit test fails, including a zero-per-evaluation or
  non-scaling-per-element contract;
- `allocs/op` grows with element count for an operator whose specification
  requires bounded traversal setup;
- a benchmark comparison performs unequal semantic work, ownership copying,
  sorting, or preallocation;
- a statistically significant timing regression of at least 10% in the
  package geomean, or at least 15% in a release-critical benchmark, has no
  reviewed explanation; or
- a material `B/op` or `allocs/op` regression has no reviewed explanation and
  is not already covered by a stricter allocation test.

Timing and byte thresholds are review policy, not cross-platform automated
failure thresholds. Hosted-runner noise, compiler changes, and new benchmarks
must be considered before classifying a result.

## Explainable regressions

A release owner may accept a timing or non-contractual memory regression when
the release record identifies the affected benchmarks, magnitude, likely
cause, and accepted tradeoff. Typical valid causes include a correctness or
resource-safety fix, changed compiler code generation, or intentionally
stronger ownership guarantees. An unexplained material regression remains
blocking.

The 10/1K/1M cases and other large-data benchmarks run only in the manual or
tag-triggered release workflow, not as a timing gate on every pull request.
Ordinary CI retains the targeted allocation tests as the fast hard gate.
