#!/usr/bin/env bash
set -e

REPO="dilipgurung/tiny-server"
NAME="tiny-server"
VERSION=""
PREFIX="${PREFIX:-/usr/local/bin}"
GITHUB="https://github.com/$REPO"

# --- Parse Args ---
while [[ $# -gt 0 ]]; do
    case "$1" in
    --version)
        VERSION="$2"
        shift 2
        ;;
    --prefix)
        PREFIX="$2"
        shift 2
        ;;
    --help|-h)
        echo "Usage: $0 [--version v1.0.0] [--prefix /usr/local/bin]"
        echo "  --version   Specific release version to install (default: latest)"
        echo "  --prefix    Install prefix directory (default: /usr/local/bin;"
        echo "              can also be set via the PREFIX env var)"
        exit 0
        ;;
    *)
        echo "❌ Unknown option: $1"
        exit 1
        ;;
    esac
done

# --- Get Latest Version if Not Set ---
if [ -z "$VERSION" ]; then
    echo "📦 Fetching latest version..."
    VERSION=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | cut -d'"' -f4)
fi

# --- Detect OS and Arch ---
OS="$(uname | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
x86_64) ARCH="amd64" ;;
arm64 | aarch64) ARCH="arm64" ;;
*)
    echo "❌ Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

# --- File Info ---
FILENAME="${NAME}_${VERSION#v}_${OS}_${ARCH}.tar.gz"
URL="$GITHUB/releases/download/$VERSION/$FILENAME"
CHECKSUM_URL="$GITHUB/releases/download/$VERSION/${NAME}_${VERSION#v}_checksums.txt"

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT
cd "$TMP_DIR"

# --- Download Files ---
echo "⬇️  Downloading $FILENAME..."
curl -fsSL -O "$URL"

echo "⬇️  Downloading checksums.txt..."
curl -fsSL -O "$CHECKSUM_URL"

# --- Verify Checksum ---
echo "🔐 Verifying checksum..."
EXPECTED=$(grep "$FILENAME" "${NAME}_${VERSION#v}_checksums.txt" | cut -d' ' -f1)
ACTUAL=$(shasum -a 256 "$FILENAME" | cut -d' ' -f1)

if [ "$EXPECTED" != "$ACTUAL" ]; then
    echo "❌ Checksum mismatch!"
    echo "Expected: $EXPECTED"
    echo "Actual:   $ACTUAL"
    exit 1
fi

# --- Extract and Install ---
echo "📂 Extracting archive..."
tar -xzf "$FILENAME"

# --- Install ---
mkdir -p "$PREFIX"
if [ -w "$PREFIX" ]; then
    echo "🚀 Installing $NAME to $PREFIX..."
    mv "$NAME" "$PREFIX/"
    chmod +x "$PREFIX/$NAME"
else
    echo "🚀 Installing $NAME to $PREFIX (requires sudo)..."
    sudo mv "$NAME" "$PREFIX/"
    sudo chmod +x "$PREFIX/$NAME"
fi

echo "✅ $NAME $VERSION installed successfully!"
echo "   Run '$PREFIX/$NAME -v' to verify."
