#!/bin/bash
set -euo pipefail

# devrec — Linux test session recorder installer
# Usage: curl -fsSL https://raw.githubusercontent.com/0embsd/devrec/main/install.sh | sudo bash

REPO="0embsd/devrec"
BIN="/usr/local/bin/devrec"
DIR="/opt/devrec"
PIDDIR="/var/run/devrec"
VERSION="${DEVREC_VERSION:-latest}"

ARCH=$(uname -m)
case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

echo "=== devrec installer ==="
echo "Arch: $ARCH"

if [ "$VERSION" = "latest" ]; then
    URL="https://github.com/$REPO/releases/latest/download/devrec-linux-$ARCH"
else
    URL="https://github.com/$REPO/releases/download/$VERSION/devrec-linux-$ARCH"
fi

echo "Downloading $URL ..."
TMP=$(mktemp)
curl -fsSL --connect-timeout 15 --max-time 120 -o "$TMP" "$URL" || {
    echo "ERROR: Download failed. Check https://github.com/$REPO/releases"
    rm -f "$TMP"
    exit 1
}

install -m 755 "$TMP" "$BIN"
rm -f "$TMP"
echo "Installed: $BIN"

mkdir -p "$DIR/sessions" "$DIR/tmp" "$PIDDIR"
chmod 755 "$PIDDIR"

echo ""
echo "=== devrec installed ==="
echo ""
echo "Quick start:"
echo "  devrec start -t 'my-test' -c kernel,resources,ports"
echo "  devrec stop"
echo "  devrec replay <session-id>"
echo "  devrec watch --interval 30s"
echo "  devrec list"
echo "  devrec status"
"$BIN" --help
