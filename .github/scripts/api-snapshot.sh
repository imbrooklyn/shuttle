#!/usr/bin/env bash

set -euo pipefail

export LC_ALL=C
export GOTOOLCHAIN=local

readonly expected_go_version='go1.27.0'
actual_go_version="$(go env GOVERSION)"
if [[ "${actual_go_version}" != "${expected_go_version}" ]]; then
  printf 'API snapshot requires %s; found %s\n' "${expected_go_version}" "${actual_go_version}" >&2
  exit 1
fi

readonly packages=(comparator optional predicate stream)
for index in "${!packages[@]}"; do
  package="${packages[index]}"
  printf '===== %s =====\n\n' "${package}"
  package_doc="$(go doc -all "./${package}")"
  printf '%s\n' "${package_doc}"
  if ((index + 1 < ${#packages[@]})); then
    printf '\n'
  fi
done
