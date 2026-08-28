#!/usr/bin/env sh

set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
WEB_ROOT="$REPO_ROOT/src/web"
SERVER_ROOT="$REPO_ROOT/src/server"
EMBED_DIR="$SERVER_ROOT/internal/webui/dist"
PACKAGING_ROOT="$REPO_ROOT/packaging/magisk"
RELEASE_ROOT="$REPO_ROOT/release"
OUTPUT_ROOT="${OUTPUT_DIR:-$RELEASE_ROOT/module-arm64}"

if [ -L "$RELEASE_ROOT" ]; then
  echo "refusing to use symlink release directory: $RELEASE_ROOT" >&2
  exit 1
fi

for command_name in go git node pnpm unzip zip; do
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

if [ -z "${VERSION_CODE:-}" ]; then
  VERSION_CORE=$(printf '%s' "$VERSION" | sed 's/[-+].*$//')
  VERSION_MAJOR=$(printf '%s' "$VERSION_CORE" | cut -d. -f1)
  VERSION_MINOR=$(printf '%s' "$VERSION_CORE" | cut -s -d. -f2)
  VERSION_PATCH=$(printf '%s' "$VERSION_CORE" | cut -s -d. -f3)
  VERSION_MINOR=${VERSION_MINOR:-0}
  VERSION_PATCH=${VERSION_PATCH:-0}
  case "$VERSION_MAJOR$VERSION_MINOR$VERSION_PATCH" in
    ''|*[!0-9]*) VERSION_CODE=1 ;;
    *)
      VERSION_CODE=$((VERSION_MAJOR * 1000000 + VERSION_MINOR * 1000 + VERSION_PATCH))
      [ "$VERSION_CODE" -gt 0 ] || VERSION_CODE=1
      ;;
  esac
fi
case "$VERSION_CODE" in
  ''|*[!0-9]*)
    echo "VERSION_CODE must be a positive integer: $VERSION_CODE" >&2
    exit 1
    ;;
esac
if [ "$VERSION_CODE" -le 0 ] || [ "$VERSION_CODE" -gt 2147483647 ]; then
  echo "VERSION_CODE is outside 1..2147483647: $VERSION_CODE" >&2
  exit 1
fi

ZIP_PATH="$RELEASE_ROOT/sing-box-observability-$VERSION-module-arm64.zip"
ZIP_CHECKSUM_PATH="$ZIP_PATH.sha256"
if [ -L "$ZIP_PATH" ] || [ -L "$ZIP_CHECKSUM_PATH" ]; then
  echo "refusing to replace symlink release archive" >&2
  exit 1
fi

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
rm -f -- "$ZIP_PATH" "$ZIP_CHECKSUM_PATH"
mkdir -p -- "$OUTPUT_ROOT/bin" "$OUTPUT_ROOT/config" "$OUTPUT_ROOT/docs" "$OUTPUT_ROOT/webroot"

MODULE=github.com/Fanju6/sing-box-observability/src/server/internal/buildinfo
LDFLAGS="-s -w -X $MODULE.Version=$VERSION -X $MODULE.Commit=$COMMIT -X $MODULE.BuildTime=$BUILD_TIME"
(cd "$SERVER_ROOT" && \
  CGO_ENABLED=0 GOOS=android GOARCH=arm64 \
  go build -buildvcs=false -tags webui -trimpath -ldflags "$LDFLAGS" \
    -o "$OUTPUT_ROOT/bin/sing-box-observability" ./cmd/sing-box-observability)

BINARY_PATH="$OUTPUT_ROOT/bin/sing-box-observability"
if ! grep -aF "$FRONTEND_MARKER" "$BINARY_PATH" >/dev/null; then
  echo "Android binary does not contain the current frontend build marker: $FRONTEND_MARKER" >&2
  exit 1
fi
if grep -aF 'mockServiceWorker.js' "$BINARY_PATH" >/dev/null; then
  echo 'Android binary unexpectedly contains mockServiceWorker.js' >&2
  exit 1
fi

sed \
  -e "s/@VERSION@/$VERSION/g" \
  -e "s/@VERSION_CODE@/$VERSION_CODE/g" \
  "$PACKAGING_ROOT/module.prop.in" >"$OUTPUT_ROOT/module.prop"
cp "$PACKAGING_ROOT/customize.sh" "$OUTPUT_ROOT/"
cp "$PACKAGING_ROOT/service.sh" "$OUTPUT_ROOT/"
cp "$PACKAGING_ROOT/action.sh" "$OUTPUT_ROOT/"
cp "$PACKAGING_ROOT/uninstall.sh" "$OUTPUT_ROOT/"
cp "$PACKAGING_ROOT/sing-box-observabilityctl" "$OUTPUT_ROOT/bin/"
cp "$PACKAGING_ROOT/config.default.yaml" "$OUTPUT_ROOT/config/default.yaml"
cp -R "$PACKAGING_ROOT/webroot/." "$OUTPUT_ROOT/webroot/"
cp "$PACKAGING_ROOT/README.md" "$OUTPUT_ROOT/docs/"
cp "$REPO_ROOT/LICENSE" "$OUTPUT_ROOT/docs/"
cp "$REPO_ROOT/NOTICE" "$OUTPUT_ROOT/docs/"
cp "$REPO_ROOT/THIRD_PARTY_LICENSES.txt" "$OUTPUT_ROOT/docs/"
chmod 755 \
  "$OUTPUT_ROOT/bin/sing-box-observability" \
  "$OUTPUT_ROOT/bin/sing-box-observabilityctl" \
  "$OUTPUT_ROOT/customize.sh" \
  "$OUTPUT_ROOT/service.sh" \
  "$OUTPUT_ROOT/action.sh" \
  "$OUTPUT_ROOT/uninstall.sh"
chmod 644 "$OUTPUT_ROOT/module.prop" "$OUTPUT_ROOT/config/default.yaml"

{
  printf 'version=%s\n' "$VERSION"
  printf 'versionCode=%s\n' "$VERSION_CODE"
  printf 'commit=%s\n' "$COMMIT"
  printf 'buildTime=%s\n' "$BUILD_TIME"
  printf 'target=magisk/android/arm64\n'
  printf 'frontendDigest=%s\n' "$FRONTEND_DIGEST"
} >"$OUTPUT_ROOT/docs/BUILD-MANIFEST.txt"

(
  cd "$WEB_ROOT/dist"
  find . -type f | LC_ALL=C sort | while IFS= read -r relative; do
    relative=${relative#./}
    printf '%s  %s\n' "$(hash_file "$relative")" "$relative"
  done
) >"$OUTPUT_ROOT/docs/FRONTEND-SHA256.txt"

echo "Collecting third-party license texts..."
node "$REPO_ROOT/scripts/collect-frontend-licenses.mjs" \
  --web-root "$WEB_ROOT" \
  --inventory "$OUTPUT_ROOT/docs/FRONTEND-LICENSES.json" \
  --notices "$OUTPUT_ROOT/docs/FRONTEND-LICENSES.txt"
go run "$REPO_ROOT/scripts/collect-go-licenses.go" \
  -server-root "$SERVER_ROOT" \
  -target ./cmd/sing-box-observability \
  -tags webui \
  -goos android \
  -goarch arm64 \
  -inventory "$OUTPUT_ROOT/docs/GO-MODULES.txt" \
  -notices "$OUTPUT_ROOT/docs/GO-LICENSES.txt"

(
  cd "$OUTPUT_ROOT"
  find . -type f ! -name MODULE-SHA256SUMS.txt | LC_ALL=C sort | while IFS= read -r relative; do
    relative=${relative#./}
    printf '%s  %s\n' "$(hash_file "$relative")" "$relative"
  done
) >"$OUTPUT_ROOT/docs/MODULE-SHA256SUMS.txt"

for script in "$OUTPUT_ROOT/customize.sh" "$OUTPUT_ROOT/service.sh" "$OUTPUT_ROOT/action.sh" "$OUTPUT_ROOT/uninstall.sh" "$OUTPUT_ROOT/bin/sing-box-observabilityctl"; do
  sh -n "$script"
done

(
  cd "$OUTPUT_ROOT"
  zip -q -9 -r "$ZIP_PATH" .
)
zip -T "$ZIP_PATH" >/dev/null
ZIP_ENTRIES=$(unzip -Z1 "$ZIP_PATH")
for required_entry in module.prop customize.sh service.sh bin/sing-box-observability config/default.yaml webroot/index.html; do
  if ! printf '%s\n' "$ZIP_ENTRIES" | grep -Fx "$required_entry" >/dev/null; then
    echo "Magisk archive is missing required entry: $required_entry" >&2
    exit 1
  fi
done
printf '%s  %s\n' "$(hash_file "$ZIP_PATH")" "$(basename -- "$ZIP_PATH")" >"$ZIP_CHECKSUM_PATH"

echo "Magisk / KernelSU module ZIP: $ZIP_PATH"
echo "ZIP checksum: $ZIP_CHECKSUM_PATH"
echo "Module staging folder: $OUTPUT_ROOT"
echo "Version: $VERSION ($VERSION_CODE)"
echo "Commit: $COMMIT"
