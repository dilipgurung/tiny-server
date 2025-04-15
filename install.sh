#!/usr/bin/env bash
set -e

REPO="dilipgurung/tiny-server"
NAME="tiny-server"
VERSION=""
GITHUB="https://github.com/$REPO"

# --- Parse Args ---
while [[ $# -gt 0 ]]; do
    case "$1" in
    --version)
        VERSION="$2"
        shift 2
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
echo $CHECKSUM_URL
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

echo "🚀 Installing $NAME to /usr/local/bin (requires sudo)..."
sudo mv "$NAME" /usr/local/bin/
sudo chmod +x /usr/local/bin/"$NAME"

echo "✅ $NAME $VERSION installed successfully!"
