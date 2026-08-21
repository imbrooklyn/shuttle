# Public API baseline

`shuttle-v1.txt` is the reviewed pre-v1 public API and GoDoc baseline for the
four runtime packages: `comparator`, `optional`, `predicate`, and `stream`. Its
source of truth is the repository commit containing the file; the initial
baseline was derived from parent commit
`06d7c21d592c9c15b0b1833427110cb3f5bb00fe` plus the reviewed `FlatMapSlice`
addition.

Regenerate it from the module root with the repository's pinned Go 1.27.0
toolchain:

```bash
bash .github/scripts/api-snapshot.sh > api/shuttle-v1.txt
```

The snapshot deliberately uses `go doc -all`: generic method signatures are
rendered by the same compiler version that builds the module, and documentation
drift is visible in the same review. CI regenerates the snapshot and requires
an exact match. Updating the file is an explicit API-review action, not a way to
suppress the gate. The review must classify every changed declaration and
synchronize `README.md`, `DESIGN.md`, `API_SPEC.md`, GoDoc, tests, and examples.
PR CI also compares the base and candidate snapshots and retains the report as
an artifact.

`golang.org/x/exp/cmd/apidiff` at
`v0.0.0-20260820122028-d6e0b57b1a69` was evaluated first, but its comparison
currently overflows the `go/types` method-set expansion stack for Shuttle's Go
1.27 generic methods. Re-evaluate it after that limitation is fixed; do not
silently replace this gate with a tool that cannot process the method set.

The snapshot checks public declarations and documentation. Behavioral
compatibility, dependency direction, and the prohibitions on reflection, hidden
errors, and implicit concurrency remain source-review and test responsibilities.
