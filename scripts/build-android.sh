#!/usr/bin/env sh

set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
WEB_ROOT="$REPO_ROOT/src/web"
SERVER_ROOT="$REPO_ROOT/src/server"
EMBED_DIR="$SERVER_ROOT/internal/webui/dist"
RELEASE_ROOT="$REPO_ROOT/release"
OUTPUT_ROOT="${OUTPUT_DIR:-$RELEASE_ROOT/android-arm64}"

if [ -L "$RELEASE_ROOT" ]; then
  echo "refusing to use symlink release directory: $RELEASE_ROOT" >&2
  exit 1
fi

for command_name in go git node pnpm; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "required command is unavailable: $command_name" >&2
    exit 1
  fi
done
if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
  echo 'required command is unavailable: sha256sum or shasum' >&2
  exit 1
fi

RELEASE_ROOT=$(node -e 'process.stdout.write(require("node:path").resolve(process.argv[1]))' "$RELEASE_ROOT")
OUTPUT_ROOT=$(node -e 'process.stdout.write(require("node:path").resolve(process.argv[1]))' "$OUTPUT_ROOT")

if [ "$(dirname -- "$OUTPUT_ROOT")" != "$RELEASE_ROOT" ]; then
    echo "OUTPUT_DIR must be a direct child of $RELEASE_ROOT" >&2
    exit 1
fi
if [ -L "$OUTPUT_ROOT" ]; then
  echo "refusing to replace symlink output directory: $OUTPUT_ROOT" >&2
  exit 1
fi

hash_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

VERSION="${VERSION:-$(git -C "$REPO_ROOT" describe --tags --always --dirty 2>/dev/null || printf '0.1.0-dev')}"
case "$VERSION" in
  v[0-9]*) VERSION=${VERSION#v} ;;
esac
case "$VERSION" in
  ''|*[!0-9A-Za-z._+-]*)
    echo "invalid VERSION: $VERSION" >&2
    exit 1
    ;;
esac
COMMIT=$(git -C "$REPO_ROOT" rev-parse --verify HEAD 2>/dev/null || printf 'unknown')
if [ -n "$(git -C "$REPO_ROOT" status --porcelain --untracked-files=normal)" ]; then
  COMMIT="${COMMIT}-dirty"
fi
BUILD_TIME=$(git -C "$REPO_ROOT" show -s --format=%cI HEAD 2>/dev/null || printf 'unknown')

echo "Building frontend with locked dependencies..."
pnpm --dir "$WEB_ROOT" install --frozen-lockfile
pnpm --dir "$WEB_ROOT" build

DIGEST_INPUT=$(mktemp "${TMPDIR:-/tmp}/sbobs-frontend-digest.XXXXXX")
trap 'rm -f "$DIGEST_INPUT"' EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
(
  cd "$WEB_ROOT/dist"
  find . -type f | LC_ALL=C sort | while IFS= read -r relative; do
    relative=${relative#./}
    printf '%s  %s\n' "$(hash_file "$relative")" "$relative"
  done
) >"$DIGEST_INPUT"
if [ ! -s "$DIGEST_INPUT" ]; then
  echo 'frontend build produced no files' >&2
  exit 1
fi
FRONTEND_DIGEST=$(hash_file "$DIGEST_INPUT")
rm -f "$DIGEST_INPUT"
trap - EXIT HUP INT TERM
FRONTEND_MARKER="frontend-$FRONTEND_DIGEST.txt"
printf '%s\n' "$FRONTEND_DIGEST" >"$WEB_ROOT/dist/$FRONTEND_MARKER"

if [ "$EMBED_DIR" != "$REPO_ROOT/src/server/internal/webui/dist" ]; then
  echo "refusing to replace unexpected embed directory: $EMBED_DIR" >&2
  exit 1
fi
if [ -L "$EMBED_DIR" ]; then
  echo "refusing to replace symlink embed directory: $EMBED_DIR" >&2
  exit 1
fi
rm -rf -- "$EMBED_DIR"
mkdir -p -- "$EMBED_DIR"
cp -R "$WEB_ROOT/dist/." "$EMBED_DIR/"

echo "Testing the embedded build..."
(cd "$SERVER_ROOT" && go test -tags webui ./...)

rm -rf -- "$OUTPUT_ROOT"
mkdir -p -- "$OUTPUT_ROOT"

MODULE=github.com/Fanju6/sing-box-observability/src/server/internal/buildinfo
LDFLAGS="-s -w -X $MODULE.Version=$VERSION -X $MODULE.Commit=$COMMIT -X $MODULE.BuildTime=$BUILD_TIME"
(cd "$SERVER_ROOT" && \
  CGO_ENABLED=0 GOOS=android GOARCH=arm64 \
  go build -buildvcs=false -tags webui -trimpath -ldflags "$LDFLAGS" \
    -o "$OUTPUT_ROOT/sing-box-observability" ./cmd/sing-box-observability)

if ! grep -aF "$FRONTEND_MARKER" "$OUTPUT_ROOT/sing-box-observability" >/dev/null; then
  echo "Android binary does not contain the current frontend build marker: $FRONTEND_MARKER" >&2
  exit 1
fi
if grep -aF 'mockServiceWorker.js' "$OUTPUT_ROOT/sing-box-observability" >/dev/null; then
  echo 'Android binary unexpectedly contains mockServiceWorker.js' >&2
  exit 1
fi

cp "$REPO_ROOT/packaging/android/config.example.yaml" "$OUTPUT_ROOT/config.yaml"
cp "$REPO_ROOT/packaging/android/sing-box-observabilityctl" "$OUTPUT_ROOT/"
cp "$REPO_ROOT/packaging/android/service.d.sh" "$OUTPUT_ROOT/"
cp "$REPO_ROOT/packaging/android/README.md" "$OUTPUT_ROOT/"
cp "$REPO_ROOT/LICENSE" "$OUTPUT_ROOT/"
cp "$REPO_ROOT/NOTICE" "$OUTPUT_ROOT/"
cp "$REPO_ROOT/THIRD_PARTY_LICENSES.txt" "$OUTPUT_ROOT/"
chmod 755 "$OUTPUT_ROOT/sing-box-observability" "$OUTPUT_ROOT/sing-box-observabilityctl" "$OUTPUT_ROOT/service.d.sh"
chmod 600 "$OUTPUT_ROOT/config.yaml"

{
  printf 'version=%s\n' "$VERSION"
  printf 'commit=%s\n' "$COMMIT"
  printf 'buildTime=%s\n' "$BUILD_TIME"
  printf 'target=android/arm64\n'
  printf 'frontendDigest=%s\n' "$FRONTEND_DIGEST"
} >"$OUTPUT_ROOT/BUILD-MANIFEST.txt"

(
  cd "$WEB_ROOT/dist"
  find . -type f | LC_ALL=C sort | while IFS= read -r relative; do
    relative=${relative#./}
    printf '%s  %s\n' "$(hash_file "$relative")" "$relative"
  done
) >"$OUTPUT_ROOT/FRONTEND-SHA256.txt"

echo "Collecting third-party license texts..."
node "$REPO_ROOT/scripts/collect-frontend-licenses.mjs" \
  --web-root "$WEB_ROOT" \
  --inventory "$OUTPUT_ROOT/FRONTEND-LICENSES.json" \
  --notices "$OUTPUT_ROOT/FRONTEND-LICENSES.txt"
go run "$REPO_ROOT/scripts/collect-go-licenses.go" \
  -server-root "$SERVER_ROOT" \
  -target ./cmd/sing-box-observability \
  -tags webui \
  -goos android \
  -goarch arm64 \
  -inventory "$OUTPUT_ROOT/GO-MODULES.txt" \
  -notices "$OUTPUT_ROOT/GO-LICENSES.txt"

(
  cd "$OUTPUT_ROOT"
  find . -type f ! -name SHA256SUMS.txt | LC_ALL=C sort | while IFS= read -r relative; do
    relative=${relative#./}
    printf '%s  %s\n' "$(hash_file "$relative")" "$relative"
  done
) >"$OUTPUT_ROOT/SHA256SUMS.txt"

echo "Android package folder: $OUTPUT_ROOT"
echo "Version: $VERSION"
echo "Commit: $COMMIT"
