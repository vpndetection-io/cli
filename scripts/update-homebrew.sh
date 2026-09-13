#!/bin/bash

# Updates the Homebrew formula in vpndetection-io/homebrew-tap for a release.
#
# Our own tap rather than homebrew-core, which requires a notability the project
# does not have yet. `brew tap vpndetection-io/tap` then `brew install
# vpndetection` is the install, and `brew upgrade` keeps it current.

set -euo pipefail

cd "$(dirname "$0")/.."

VSN="${1:?usage: update-homebrew.sh <version>}"
TAP_TOKEN="${TAP_TOKEN:?a token with push access to the tap is required}"
REPO="vpndetection-io/homebrew-tap"
BASE="https://github.com/vpndetection-io/cli/releases/download/v${VSN}"

# Hashes are taken from the release's own artifacts rather than from the local
# build, so the formula describes what people will actually download.
function sha_of() {
    curl -fsSL "${BASE}/$1" | sha256sum | cut -d' ' -f1
}

echo "==> Hashing release artifacts..."
DARWIN_ARM="$(sha_of "vpndetection_${VSN}_darwin_arm64.tar.gz")"
DARWIN_AMD="$(sha_of "vpndetection_${VSN}_darwin_amd64.tar.gz")"
LINUX_ARM="$(sha_of "vpndetection_${VSN}_linux_arm64.tar.gz")"
LINUX_AMD="$(sha_of "vpndetection_${VSN}_linux_amd64.tar.gz")"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
git clone -q "https://x-access-token:${TAP_TOKEN}@github.com/${REPO}.git" "$TMP/tap"

mkdir -p "$TMP/tap/Formula"
sed -e "s|@VSN@|${VSN}|g" \
    -e "s|@DARWIN_ARM@|${DARWIN_ARM}|g" \
    -e "s|@DARWIN_AMD@|${DARWIN_AMD}|g" \
    -e "s|@LINUX_ARM@|${LINUX_ARM}|g" \
    -e "s|@LINUX_AMD@|${LINUX_AMD}|g" \
    homebrew/vpndetection.rb.tmpl > "$TMP/tap/Formula/vpndetection.rb"

cd "$TMP/tap"
if git diff --quiet ; then
    echo "==> Formula already at ${VSN}; nothing to do."
    exit 0
fi
git -c user.name="vpndetection-bot" -c user.email="support@vpndetection.io" \
    commit -qam "vpndetection ${VSN}"
git push -q origin HEAD
echo "==> ${REPO} updated to ${VSN}"
