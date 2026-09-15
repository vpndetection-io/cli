module github.com/vpndetection-io/cli

go 1.25.0

// x/term and x/sys are held BELOW their latest releases on purpose: from
// x/term v0.46.0 and x/sys v0.48.0 they declare `go 1.26.0`, which would
// raise this module's floor to a toolchain the build images do not have yet
// and fail every cross-compile with "requires go >= 1.26.0". Raise them only
// together with the Go version in the Dockerfile and the CI matrix.
//
// The last version of each that still declares go 1.25 is x/term v0.45.0 and
// x/sys v0.47.0 (checked 2026-09-15), so there is headroom under the floor and
// the pins below are not the ceiling.
require (
	github.com/fatih/color v1.18.0
	github.com/mslmio/libgo-complete v1.0.0
	github.com/mslmio/libgo-iputil v1.0.0
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c
	github.com/spf13/pflag v1.0.10
	go.etcd.io/bbolt v1.4.3
	golang.org/x/term v0.35.0
	gopkg.in/yaml.v3 v3.0.1
)

require github.com/vpndetection-io/sdk-go/v5 v5.1.0

require (
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/oapi-codegen/runtime v1.7.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
)
