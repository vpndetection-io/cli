#!/bin/sh

# Installs the VPNDetection CLI on macOS.
#
#   curl -Ls https://github.com/vpndetection-io/cli/releases/latest/download/macos.sh | sh

set -e

VSN="${VSN:-0.1.0}"
case "$(uname -m)" in
    arm64)  ARCH=arm64 ;;
    x86_64) ARCH=amd64 ;;
    *)      echo "unsupported architecture: $(uname -m)" >&2 ; exit 1 ;;
esac

TARBALL="vpndetection_${VSN}_darwin_${ARCH}.tar.gz"
URL="https://github.com/vpndetection-io/cli/releases/download/v${VSN}/${TARBALL}"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "downloading ${TARBALL}..."
curl -fLs "$URL" -o "${TMP}/${TARBALL}"
tar -xzf "${TMP}/${TARBALL}" -C "$TMP"

# /usr/local/bin needs root on most machines and does not on some; only ask
# when it is actually needed.
if [ -w /usr/local/bin ] ; then
    mv "${TMP}/vpndetection" /usr/local/bin/vpndetection
else
    sudo mv "${TMP}/vpndetection" /usr/local/bin/vpndetection
fi

echo
echo "installed. run 'vpndetection --help', or 'vpndetection completion install'"
echo "for shell auto-completion."
echo
echo "macOS may refuse to run it the first time: the binary is not notarized."
echo "Allow it under System Settings > Privacy & Security."
