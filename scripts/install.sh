#!/bin/sh
set -eu

REPOSITORY="pabloLopezSanchezz/gutil"
INSTALL_DIR="${GUTIL_INSTALL_DIR:-$HOME/.local/bin}"

if [ -n "${GUTIL_VERSION:-}" ]; then
  VERSION="$GUTIL_VERSION"
else
  VERSION=$(curl -fsSL "https://api.github.com/repos/$REPOSITORY/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1)
fi

[ -n "$VERSION" ] || { echo "Could not determine the latest gUtil version." >&2; exit 1; }

case "$(uname -s)" in
  Darwin) OS="darwin" ;;
  Linux) OS="linux" ;;
  *) echo "Unsupported operating system: $(uname -s)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  arm64|aarch64) ARCH="arm64" ;;
  x86_64|amd64) ARCH="amd64" ;;
  *) echo "Unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

ARCHIVE="gutil_${VERSION}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/$REPOSITORY/releases/download/$VERSION"
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT INT TERM

curl -fsSL "$BASE_URL/$ARCHIVE" -o "$TEMP_DIR/$ARCHIVE"
curl -fsSL "$BASE_URL/checksums.txt" -o "$TEMP_DIR/checksums.txt"

EXPECTED=$(awk -v name="$ARCHIVE" '$2 == name || $2 == "*" name { print $1 }' "$TEMP_DIR/checksums.txt")
[ -n "$EXPECTED" ] || { echo "No checksum found for $ARCHIVE." >&2; exit 1; }

if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL=$(sha256sum "$TEMP_DIR/$ARCHIVE" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL=$(shasum -a 256 "$TEMP_DIR/$ARCHIVE" | awk '{print $1}')
else
  echo "A sha256 checksum tool is required." >&2
  exit 1
fi

[ "$EXPECTED" = "$ACTUAL" ] || { echo "Checksum verification failed for $ARCHIVE." >&2; exit 1; }
tar -xzf "$TEMP_DIR/$ARCHIVE" -C "$TEMP_DIR"
chmod +x "$TEMP_DIR/gutil"
mkdir -p "$INSTALL_DIR"
mv "$TEMP_DIR/gutil" "$INSTALL_DIR/gutil.new"
mv "$INSTALL_DIR/gutil.new" "$INSTALL_DIR/gutil"

echo "Installed gUtil $VERSION to $INSTALL_DIR/gutil"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "Add $INSTALL_DIR to PATH, then open a new terminal." ;;
esac
