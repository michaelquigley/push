#!/usr/bin/env bash
#
# compute version string from git state.
# reads GITHUB_REF and GITHUB_REF_NAME from environment.
# outputs version to stdout.
#
set -euo pipefail

SHORT=$(git rev-parse HEAD | cut -c1-8)

if [[ "${GITHUB_REF}" == refs/tags/v* ]]; then
  echo "${GITHUB_REF_NAME}"
elif TAG=$(git tag --points-at HEAD | grep -E '^v[0-9]' | head -1) && [ -n "${TAG}" ]; then
  echo "${TAG}"
else
  LATEST_TAG=$(git describe --tags --match 'v[0-9]*' --abbrev=0 2>/dev/null || true)
  if [ -n "${LATEST_TAG}" ]; then
    BASE=$(echo "${LATEST_TAG}" | sed 's/^v//' | awk -F. '{printf "v%s.%s.%s", $1, $2, $3+1}')
  else
    BASE="v0.0.0"
  fi
  DATE=$(date -u +%Y%m%d)
  echo "${BASE}-dev.${DATE}.${SHORT}"
fi
