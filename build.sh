#!/bin/bash
set -e

# Get version information
VERSION=${VERSION:-$(git describe --tags --exact-match 2>/dev/null || echo "dev")}
COMMIT=${COMMIT:-$(git rev-parse HEAD)}
BUILD_TIME=${BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}

echo "Building CringeSweeper ${VERSION}"
echo "Commit: ${COMMIT}"
echo "Build Time: ${BUILD_TIME}"

# Build with version information
go build -ldflags "\
  -X github.com/gerrowadat/cringesweeper/internal.Version=${VERSION} \
  -X github.com/gerrowadat/cringesweeper/internal.Commit=${COMMIT} \
  -X github.com/gerrowadat/cringesweeper/internal.BuildTime=${BUILD_TIME}" \
  -o cringesweeper .

echo "✅ Build complete: cringesweeper"
./cringesweeper version