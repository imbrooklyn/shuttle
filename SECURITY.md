# Security policy

## Supported versions

Until Shuttle publishes its first tag, no version is supported for production
use. Security reports against the default branch are nevertheless welcome.

After the first public release, the latest published version receives security
fixes. Older pre-v1 versions are unsupported unless a release note explicitly
states otherwise.

## Reporting a vulnerability

Do not open a public issue for a suspected vulnerability. Prefer GitHub's
[private vulnerability reporting](https://github.com/imbrooklyn/shuttle/security/advisories/new).
This is the project's only channel for receiving vulnerability details. If the
form is temporarily unavailable, do not disclose the details in an issue,
discussion, pull request, or other public channel.

Include the affected version or commit, impact, minimal reproduction, and any
known mitigation. Do not include credentials, tokens, personal data, or other
unrelated secrets.

Maintainers will acknowledge reports as soon as practical, validate impact,
coordinate a fix and disclosure date with the reporter, and credit the reporter
when requested. Public disclosure should wait until a fixed version or agreed
mitigation is available.

## Scope

Reports about Shuttle's runtime packages, release workflows, or published
module integrity are in scope. Vulnerabilities in the Go toolchain, standard
library, GitHub Actions, or analysis tools should also be reported upstream;
please notify Shuttle when they materially affect a supported release.
