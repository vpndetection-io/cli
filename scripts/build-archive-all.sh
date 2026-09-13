#!/bin/bash

# Builds every platform and packages each one for release: tar.gz everywhere,
# zip on Windows, and a .deb per Linux architecture.

set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$PWD"

VSN="${1:-$(grep -m1 'var version = ' main.go | cut -d'"' -f2)}"
LINUX_ONLY="${2:-false}"

./scripts/build-all-platforms.sh "$VSN" "$LINUX_ONLY"

cd "$ROOT/build"
for f in vpndetection_"${VSN}"_* ; do
    case "$f" in
        *.tar.gz|*.zip|*.deb) continue ;;
    esac
    if [[ "$f" == *windows* ]] ; then
        # Named plainly inside the archive, or the extracted file is
        # vpndetection_1.2.3_windows_amd64.exe and the `vpndetection` command
        # does not exist after install. winget assumes the plain name.
        cp "$f" vpndetection.exe
        zip -q "${f%.exe}.zip" vpndetection.exe
        rm vpndetection.exe
    else
        cp "$f" vpndetection
        tar -czf "${f}.tar.gz" vpndetection
        rm vpndetection
    fi
done
cd "$ROOT"

# Debian packages. The control file is a template; version and architecture are
# filled per build.
declare -A DEB_ARCH=(
    [linux_386]=i386 [linux_amd64]=amd64
    [linux_arm]=armhf [linux_arm64]=arm64
)
for target in "${!DEB_ARCH[@]}" ; do
    bin="build/vpndetection_${VSN}_${target}"
    [ -f "$bin" ] || continue

    pkgroot="build/deb_${target}"
    rm -rf "$pkgroot"
    mkdir -p "$pkgroot/DEBIAN" "$pkgroot/usr/local/bin"
    sed -e "s/^Version: .*/Version: ${VSN}/" \
        -e "s/^Architecture: .*/Architecture: ${DEB_ARCH[$target]}/" \
        dist/DEBIAN/control > "$pkgroot/DEBIAN/control"
    cp "$bin" "$pkgroot/usr/local/bin/vpndetection"
    dpkg-deb -Zgzip --build "$pkgroot" "build/vpndetection_${VSN}_${target}.deb" >/dev/null
    rm -rf "$pkgroot"
done

echo "packaged:"
ls -1 build/*.tar.gz build/*.zip build/*.deb 2>/dev/null | sed 's/^/    /'
