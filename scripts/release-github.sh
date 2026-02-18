#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

TAG="v0.1.1"
DO_TESTS=1
DO_UPLOAD=1
DO_PUSH=0
SKIP_WEB=0
ALLOW_DIRTY=0
GOCACHE_DIR="${GOCACHE:-/tmp/machinemon-go-cache}"

usage() {
  cat <<'EOF'
Usage: scripts/release-github.sh [options]

Builds client/server binaries, packages release archives, and uploads assets to a GitHub release.

Options:
  --tag <tag>       GitHub release tag to upload/create (default: v0.1.1)
  --push            Push current branch before building
  --no-tests        Skip go test ./...
  --no-upload       Build/package only, do not upload to GitHub release
  --skip-web        Skip npm web build (reuse existing cmd/machinemon-server/web_dist)
  --allow-dirty     Allow running with uncommitted changes
  -h, --help        Show this help
EOF
}

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Error: required command not found: $cmd" >&2
    exit 1
  fi
}

retry_cmd() {
  local attempts="$1"
  shift

  local n=1
  until "$@"; do
    if [[ "$n" -ge "$attempts" ]]; then
      return 1
    fi
    echo "Command failed (attempt $n/$attempts), retrying: $*" >&2
    n=$((n + 1))
    sleep 2
  done
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --tag)
      TAG="${2:-}"
      if [[ -z "$TAG" ]]; then
        echo "Error: --tag requires a value" >&2
        exit 1
      fi
      shift 2
      ;;
    --push)
      DO_PUSH=1
      shift
      ;;
    --no-tests)
      DO_TESTS=0
      shift
      ;;
    --no-upload)
      DO_UPLOAD=0
      shift
      ;;
    --skip-web)
      SKIP_WEB=1
      shift
      ;;
    --allow-dirty)
      ALLOW_DIRTY=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Error: unknown option: $1" >&2
      usage
      exit 1
      ;;
  esac
done

require_cmd git
require_cmd go
require_cmd tar
require_cmd shasum

if [[ "$DO_UPLOAD" -eq 1 ]]; then
  require_cmd gh
fi

if [[ "$SKIP_WEB" -eq 0 ]]; then
  require_cmd npm
fi

if [[ "$ALLOW_DIRTY" -eq 0 ]]; then
  if [[ -n "$(git status --porcelain)" ]]; then
    echo "Error: working tree is dirty. Commit/stash changes or pass --allow-dirty." >&2
    exit 1
  fi
fi

if [[ "$DO_PUSH" -eq 1 ]]; then
  echo "==> Pushing current branch"
  git push
fi

if [[ "$DO_TESTS" -eq 1 ]]; then
  echo "==> Running tests"
  GOCACHE="$GOCACHE_DIR" go test ./...
fi

if [[ "$SKIP_WEB" -eq 0 ]]; then
  echo "==> Building web app"
  pushd web >/dev/null
  if [[ -f package-lock.json ]]; then
    npm ci || npm install
  else
    npm install
  fi
  npm run build
  popd >/dev/null

  rm -rf cmd/machinemon-server/web_dist
  cp -R web/dist cmd/machinemon-server/web_dist
else
  if [[ ! -d cmd/machinemon-server/web_dist ]]; then
    echo "Error: --skip-web was set but cmd/machinemon-server/web_dist is missing" >&2
    exit 1
  fi
fi

echo "==> Cleaning dist/"
rm -f dist/machinemon-client-* dist/machinemon-server-* dist/checksums.txt

VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
TARGET_COMMIT="$(git rev-parse HEAD 2>/dev/null || echo unknown)"
BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
LDFLAGS="-s -w \
  -X github.com/machinemon/machinemon/internal/version.Version=${VERSION} \
  -X github.com/machinemon/machinemon/internal/version.Commit=${COMMIT} \
  -X github.com/machinemon/machinemon/internal/version.BuildTime=${BUILD_TIME}"

build_bin() {
  local out="$1"
  local goos="$2"
  local goarch="$3"
  local target="$4"
  local goarm="${5:-}"

  echo "==> Building $out"
  if [[ -n "$goarm" ]]; then
    GOCACHE="$GOCACHE_DIR" CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" GOARM="$goarm" \
      go build -ldflags "$LDFLAGS" -o "$out" "$target"
  else
    GOCACHE="$GOCACHE_DIR" CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
      go build -ldflags "$LDFLAGS" -o "$out" "$target"
  fi
}

build_bin dist/machinemon-client-linux-armv6 linux arm ./cmd/machinemon-client 6
build_bin dist/machinemon-client-linux-armv7 linux arm ./cmd/machinemon-client 7
build_bin dist/machinemon-client-linux-arm64 linux arm64 ./cmd/machinemon-client
build_bin dist/machinemon-client-linux-amd64 linux amd64 ./cmd/machinemon-client
build_bin dist/machinemon-client-darwin-amd64 darwin amd64 ./cmd/machinemon-client
build_bin dist/machinemon-client-darwin-arm64 darwin arm64 ./cmd/machinemon-client

build_bin dist/machinemon-server-linux-amd64 linux amd64 ./cmd/machinemon-server
build_bin dist/machinemon-server-linux-arm64 linux arm64 ./cmd/machinemon-server
build_bin dist/machinemon-server-darwin-amd64 darwin amd64 ./cmd/machinemon-server
build_bin dist/machinemon-server-darwin-arm64 darwin arm64 ./cmd/machinemon-server

echo "==> Packaging archives"
(
  cd dist
  for f in machinemon-client-* machinemon-server-*; do
    case "$f" in
      *.tar.gz) continue ;;
    esac
    COPYFILE_DISABLE=1 tar --no-xattrs --no-mac-metadata -czf "${f}.tar.gz" "$f"
  done
  shasum -a 256 *.tar.gz > checksums.txt
)

if [[ "$DO_UPLOAD" -eq 1 ]]; then
  echo "==> Publishing release assets to ${TAG}"
  assets=(
    dist/machinemon-client-darwin-amd64.tar.gz
    dist/machinemon-client-darwin-arm64.tar.gz
    dist/machinemon-client-linux-amd64.tar.gz
    dist/machinemon-client-linux-arm64.tar.gz
    dist/machinemon-client-linux-armv6.tar.gz
    dist/machinemon-client-linux-armv7.tar.gz
    dist/machinemon-server-darwin-amd64.tar.gz
    dist/machinemon-server-darwin-arm64.tar.gz
    dist/machinemon-server-linux-amd64.tar.gz
    dist/machinemon-server-linux-arm64.tar.gz
    dist/checksums.txt
  )

  if retry_cmd 3 gh release view "$TAG" >/dev/null 2>&1; then
    retry_cmd 3 gh release upload "$TAG" "${assets[@]}" --clobber
  else
    echo "Could not confirm whether tag ${TAG} release exists; trying upload first."
    if ! retry_cmd 2 gh release upload "$TAG" "${assets[@]}" --clobber; then
      retry_cmd 3 gh release create "$TAG" "${assets[@]}" --target "$TARGET_COMMIT" --title "$TAG" --generate-notes
    fi
  fi
fi

echo "==> Done"
echo "Commit: $COMMIT"
echo "Version: $VERSION"
echo "Build time: $BUILD_TIME"
