#!/bin/bash

# Sets the Docker Hub page's short description and overview, which pushing an
# image never does, then reads the page back signed out: a private repository
# is a channel nobody can find.

set -euo pipefail

cd "$(dirname "$0")/.."

DOCKERHUB_USERNAME="${DOCKERHUB_USERNAME:?the Docker Hub account to sign in as is required}"
DOCKERHUB_TOKEN="${DOCKERHUB_TOKEN:?an access token with Read, Write, Delete scope is required}"
REPO="vpndetection/cli"
API="https://hub.docker.com/v2"
SHORT="Official CLI for the VPNDetection API. Look up VPNs, proxies, Tor, hosting and CDN ranges."
OVERVIEW="dockerhub/overview.md"
TMP=""

function main() {
    if [ "$(printf '%s' "$SHORT" | wc -c)" -gt 100 ] ; then
        echo "the short description is over Docker Hub's limit of 100 bytes" >&2
        exit 1
    fi
    TMP="$(mktemp -d)"
    trap 'rm -rf "$TMP"' EXIT

    echo "==> Signing in to Docker Hub as ${DOCKERHUB_USERNAME}..."
    jq -n '{identifier: env.DOCKERHUB_USERNAME, secret: env.DOCKERHUB_TOKEN}' > "$TMP/login.json"
    hub POST "${API}/auth/token" -H 'Content-Type: application/json' \
        --data-binary "@${TMP}/login.json" > "$TMP/token.json"
    local token
    token="$(jq -r '.access_token // empty' "$TMP/token.json")"
    [ -n "$token" ] || { echo "Docker Hub signed in but answered no access_token" >&2; exit 1; }
    if [ -n "${GITHUB_ACTIONS:-}" ] ; then
        echo "::add-mask::${token}"
    fi
    # A header file keeps the token off curl's command line.
    printf 'Authorization: Bearer %s\n' "$token" > "$TMP/auth"

    echo "==> Setting ${REPO}'s description and overview..."
    jq -n --arg d "$SHORT" --rawfile f "$OVERVIEW" '{description: $d, full_description: $f}' \
        > "$TMP/page.json"
    hub PATCH "${API}/repositories/${REPO}/" -H "@${TMP}/auth" -H 'Content-Type: application/json' \
        --data-binary "@${TMP}/page.json" > /dev/null

    echo "==> Reading the page back signed out..."
    if ! hub GET "${API}/repositories/${REPO}/" > "$TMP/public.json" ; then
        echo "${REPO} is not public, so nobody signed out can see it" >&2
        exit 1
    fi
    if ! jq -e --arg d "$SHORT" --rawfile f "$OVERVIEW" \
        '.is_private == false and .description == $d
            and (.full_description | rtrimstr("\n")) == ($f | rtrimstr("\n"))' \
        "$TMP/public.json" > /dev/null ; then
        echo "${REPO}'s public page does not show what was set:" >&2
        jq '{is_private, description, full_description}' "$TMP/public.json" >&2
        exit 1
    fi
    echo "==> https://hub.docker.com/r/${REPO} shows the description and overview"
}

# Prints the answer, or fails with its status and body: curl -f would drop the
# body, which is where Docker Hub says what it refused.
function hub() {
    local method="$1" url="$2" code
    shift 2
    code="$(curl -sS -o "$TMP/answer" -w '%{http_code}' -X "$method" "$@" "$url")"
    if [ "${code:0:1}" != 2 ] ; then
        echo "${method} ${url} answered HTTP ${code}: $(head -c 500 "$TMP/answer")" >&2
        return 1
    fi
    cat "$TMP/answer"
}

main "$@"
