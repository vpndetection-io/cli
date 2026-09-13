FROM golang:1.25 AS builder

WORKDIR /src

# Every dependency is public, so no GOPRIVATE and no credential mount.
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,source=go.sum,target=go.sum \
    --mount=type=bind,source=go.mod,target=go.mod \
    go mod download -x

ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=bind,target=. \
    CGO_ENABLED=0 go build -trimpath \
        -ldflags "-s -w -X main.version=${VERSION}" \
        -o /dist/vpndetection .

# Distroless rather than a full base: this is one static binary that makes HTTPS
# calls, so all it needs is CA certificates. It does need a writable HOME for
# the config and cache, which the nonroot image provides at /home/nonroot.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /dist/vpndetection /usr/local/bin/vpndetection

ENTRYPOINT ["/usr/local/bin/vpndetection"]
