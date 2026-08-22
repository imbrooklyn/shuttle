# Release process

This document is the operational checklist for Shuttle release owners. The
normative v1 gates remain in [API_SPEC.md](API_SPEC.md), and benchmark
classification follows [BENCHMARKS.md](BENCHMARKS.md).

Public tags are immutable. Never move or delete a published tag to repair a
failed release; publish a new SemVer version instead.

## 1. Prepare a candidate

1. Choose the version. Use `v0.x.y` while the API is still gathering feedback;
   reserve `v1.0.0-rc.N` for an API-frozen v1 candidate.
2. Create a release branch from an up-to-date `main` and keep its candidate SHA
   fixed during qualification.
3. Move the entries under `Unreleased` in [CHANGELOG.md](CHANGELOG.md) into a
   heading for the chosen version and date. Start a new empty `Unreleased`
   section.
4. Confirm that public declaration, behavior, documentation, tests, and examples
   agree. Regenerate `api/shuttle-v1.txt` only after an explicit API review.
5. Confirm that CI and documentation pin the same stable Go patch and current
   stable analysis tools.

The candidate worktree must be clean, and the recorded SHA must be the commit
that will receive the tag:

```bash
go version
git status --short
git rev-parse HEAD
```

## 2. Pass ordinary CI

Push the release branch and wait for `CI Gate` on the exact candidate SHA. The
gate requires every native test, race test, cross-compile, bounded fuzz,
analysis, vulnerability, and public-API job to succeed.

For a local preflight with the pinned Go toolchain, run:

```bash
go test ./...
go test -race ./...
go vet ./...
bash .github/scripts/api-snapshot.sh > /tmp/shuttle-candidate-api.txt
diff -u api/shuttle-v1.txt /tmp/shuttle-candidate-api.txt
```

Release tools must be installed after selecting the pinned Go toolchain. A tool
binary built with an older Go version may not parse Go 1.27 generic methods. The
versions in `.github/workflows/ci.yml` are the source of truth.

## 3. Run release fuzzing

Manually dispatch `.github/workflows/release-fuzz.yml` against the fixed release
branch. Supply the full candidate commit as `expected_sha` and select at least
`1m` per target (`5m` is available for higher-risk releases). The workflow
validates that the selected ref resolves to the expected commit and runs every
committed fuzz target independently. Record the successful workflow URL.

The 10-second, two-architecture fuzz jobs in ordinary CI remain fast smoke
coverage; they do not replace this release qualification.

Do not create the tag until `Release Fuzz Gate` succeeds on the candidate SHA.

## 4. Qualify performance before tagging

Manually dispatch `.github/workflows/benchmarks.yml` against the fixed release
branch. Supply the full candidate commit as `expected_sha` and the previous
public tag as `base_ref`; for the first release, use the recorded initial
baseline commit explicitly.

Review the retained `baseline.txt`, `candidate.txt`, and `benchstat.txt` under
the policy in [BENCHMARKS.md](BENCHMARKS.md). Record every accepted material
regression and its rationale. A successful workflow without that human review
does not satisfy the release gate.

Do not create the tag until this step is complete.

## 5. Record approval

Prepare the GitHub release as a draft. Its notes must identify:

- version and exact candidate SHA;
- ordinary CI run URL;
- release fuzz run URL and per-target duration;
- pre-tag benchmark run URL, baseline, and classification;
- vulnerability database timestamp;
- public API diff classification;
- accepted regressions or known limitations; and
- the matching changelog section.

For `v1.0.0` and later, explicitly confirm every item in the Release Definition
of Done in [API_SPEC.md](API_SPEC.md).

## 6. Publish

Create a signed annotated tag on the approved SHA and push only that tag:

```bash
version=v0.1.0
candidate_sha=0123456789abcdef0123456789abcdef01234567
git tag -s "${version}" "${candidate_sha}" -m "Shuttle ${version}"
git push origin "refs/tags/${version}"
```

Replace the example values; never infer the target from a moving branch at this
stage. Publish the prepared GitHub release only after the tag is visible. The
tag-triggered benchmark workflow is a post-publication audit and must resolve to
the same candidate SHA as the pre-tag run.

## 7. Verify distribution

From a clean temporary module, verify that the public Go proxy can resolve the
new tag and enumerate every package:

```bash
version=v0.1.0
scratch_dir="$(mktemp -d)"
cd "${scratch_dir}"
go mod init shuttle-release-smoke
go get "github.com/imbrooklyn/shuttle@${version}"
go list -m "github.com/imbrooklyn/shuttle@${version}"
go list github.com/imbrooklyn/shuttle/...
```

Confirm that pkg.go.dev indexes the four runtime packages and that the
tag-triggered benchmark audit succeeds. If a defect is found after publication,
openly document it and issue the next version; do not rewrite the tag.
