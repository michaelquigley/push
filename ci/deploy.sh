#!/usr/bin/env bash
#
# deploy binaries to a push depot.
# usage: deploy.sh <binary> [binary...]
# reads DEPOT, PLATFORM, PROJECT, and KEEP from environment.
#
set -euo pipefail

if [ $# -eq 0 ]; then
  echo "usage: deploy.sh <binary> [binary...]" >&2
  exit 1
fi

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

DEPOT="${DEPOT:?DEPOT must be set (e.g. an absolute depot root); refusing to guess}"
KEEP="${KEEP:-5}"
PROJECT="${PROJECT:-$(basename "${GITHUB_REPOSITORY}")}"
PLATFORM="${PLATFORM:?PLATFORM must be set, e.g. linux-amd64}"

COMMIT=$(git rev-parse HEAD)
SHORT=$(echo "${COMMIT}" | cut -c1-8)
VERSION=$("${SCRIPT_DIR}/version.sh")

# check for existing tagged version in depot
if [[ "${GITHUB_REF}" != refs/tags/* ]]; then
  BUILD_JSON="${DEPOT}/${PROJECT}/builds/${SHORT}/build.json"
  if [ -f "${BUILD_JSON}" ]; then
    EXISTING=$(jq -r .version "${BUILD_JSON}" 2>/dev/null || true)
    if [ -n "${EXISTING}" ] && [[ "${EXISTING}" != *-dev* ]]; then
      echo "tagged version '${EXISTING}' already in depot for '${SHORT}', skipping deploy"
      exit 0
    fi
  fi
fi

# publish binaries
PLATFORM_DIR="${DEPOT}/${PROJECT}/builds/${SHORT}/${PLATFORM}"
mkdir -p "${PLATFORM_DIR}"
for bin in "$@"; do
  cp "${bin}" "${PLATFORM_DIR}/"
done

# write metadata and update latest
MESSAGE=$(git log -1 --pretty=%s)
AUTHOR=$(git log -1 --pretty=%an)
TIMESTAMP=$(git log -1 --pretty=%cI)
REF=${GITHUB_REF_NAME}

BUILD_DIR="${DEPOT}/${PROJECT}/builds/${SHORT}"
mkdir -p "${BUILD_DIR}"

jq -n   --arg sha "${COMMIT}"   --arg short "${SHORT}"   --arg message "${MESSAGE}"   --arg author "${AUTHOR}"   --arg time "${TIMESTAMP}"   --arg ref "${REF}"   --arg version "${VERSION}"   '{sha: $sha, short: $short, message: $message,
    author: $author, time: $time, ref: $ref, version: $version}'   > "${BUILD_DIR}/build.json"

echo "${SHORT}.$(date +%s)" > "${DEPOT}/${PROJECT}/latest"

# prune old builds
BUILDS_DIR="${DEPOT}/${PROJECT}/builds"
if [ -d "${BUILDS_DIR}" ]; then
  cd "${BUILDS_DIR}"
  ls -1t | tail -n +$((KEEP + 1)) | while read -r old; do
    rm -rf "${old}"
  done
fi
