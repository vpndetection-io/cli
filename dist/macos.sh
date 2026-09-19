#!/bin/sh

# Installs the VPNDetection CLI on macOS.
#
#   curl -Ls https://github.com/vpndetection-io/cli/releases/latest/download/macos.sh | sh

set -e

VSN="${VSN:-1.2.0}"

# Go 1.27 builds the release and needs macOS 13 Ventura. 1.2.0, built on Go
# 1.25, is the last release that runs on macOS 12.
os="$(sw_vers -productVersion 2>/dev/null || true)"
if [ "${os%%.*}" -lt 13 ] 2>/dev/null ; then
    echo "vpndetection needs macOS 13 Ventura or later; this is macOS ${os}." >&2
    echo "1.2.0 is the last release that runs on it:" >&2
    echo >&2
    echo "  curl -Ls https://github.com/vpndetection-io/cli/releases/download/v1.2.0/macos.sh | sh" >&2
    exit 1
fi

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
