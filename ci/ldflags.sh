#!/usr/bin/env bash
#
# compute go build ldflags string.
# reads GITHUB_REF, GITHUB_REF_NAME, GITHUB_REPOSITORY from environment.
# outputs ldflags string to stdout.
#
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

PROJECT=$(basename "${GITHUB_REPOSITORY}")
COMMIT=$(git rev-parse HEAD)
VERSION=$("${SCRIPT_DIR}/version.sh")
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
PKG="github.com/michaelquigley/push/build"

BUILDER="$(uname -s | tr '[:upper:]' '[:lower:]')/$(uname -m)"
if [ -f /etc/os-release ]; then
  BUILDER="${BUILDER} ($(. /etc/os-release && echo "${NAME} ${VERSION_ID}"))"
fi

LDFLAGS="-s -w"
LDFLAGS="${LDFLAGS} -X ${PKG}.Version=${VERSION}"
LDFLAGS="${LDFLAGS} -X ${PKG}.Hash=${COMMIT}"
LDFLAGS="${LDFLAGS} -X ${PKG}.Date=${DATE}"
LDFLAGS="${LDFLAGS} -X '${PKG}.Builder=${BUILDER}'"
LDFLAGS="${LDFLAGS} -X ${PKG}.Branch=${GITHUB_REF_NAME}"
LDFLAGS="${LDFLAGS} -X ${PKG}.CGO=${CGO_ENABLED:-0}"

echo "${LDFLAGS}"
