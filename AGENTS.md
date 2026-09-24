# vpndetection/cli/vpndetection

The official CLI, a presentation layer over `github.com/vpndetection-io/sdk-go`.
Public repo: `github.com/vpndetection-io/cli`, with gitea as a second push URL.

Structure follows `github.com/ipinfo/cli` on purpose: a flat dispatch in
`main.go`, one `cmd_<name>.go` per command, per-command completion predictors in
`completions.go`, and output machinery in `lib/`.

## The non-obvious parts

**The cache is bucketed by CREDENTIAL, not just by address.** Which fields an
answer carries is decided by the plan behind the key, so a cache keyed on the
address alone hands a max-tier caller a free-tier answer - which reads as "not
flagged" rather than "not included". That is the absent-versus-false trap served
from our own disk. The bucket is `sha256(key)[:12] + "|" + host`; the key itself
never reaches disk. `cache_test.go` pins it through `NewClient`, not only in
`cacheBucket`: a test of that function alone stayed green with the cache opened
under an empty key, which is one bucket for every credential. The `[:12]` is
pinned in `config_test.go` instead, because every test here passes on a
one-character fingerprint - the two keys they sample differ at any width.

**Bulk is chunked, never collected.** `runLookup` pulls addresses through
`iputil.WalkAddrs` into batches of 10,000 and emits each batch before reading
the next, so memory is bounded by the chunk rather than by what was typed. A
single `8.0.0.0/8` argument is 16.7 million addresses. The SDK's own
in-memory cache is switched OFF for the same reason; the disk cache is the one
that survives a process anyway.

**One address or many is decided by the resolved COUNT**, not by argument shape.
`10.0.0.1/32` is one and prints the readable block; `10.0.0.0/30` is four and
prints JSON. The writer is opened only once the first chunk is in hand, which is
what makes that decidable without buffering the whole input.

**The subcommand is found by scanning, not by `os.Args[1]`.** `vpndetection
--session work db list` is natural to type, and reading the second argument
literally makes it a lookup of the address `--session`. `commandArg` skips the
value of each value-taking flag, so a session named `database` is not mistaken
for the command.

**The only container image is `ghcr.io/vpndetection-io/cli`**, built by the
release workflow. There is no second build targeting a private registry: a
script naming one cannot live in a public repo, and the pre-push leak gate
refuses it. Anything internal that wants the binary pulls the public image.

## Gotchas hit building it

- **Two Go versions: the BUILD and the FLOOR.** The Dockerfile, `release.yml`,
  and ci.yml's cross and govulncheck legs build on 1.27; move all four together
  when it leaves support, since govulncheck judges the Go that ships, and read
  the new line's Ports notes first. The oldest macOS the build Go runs on (13
  Ventura for 1.27) is stated in three more places that move with it: the
  README, `dist/macos.sh` and the formula template's `depends_on macos:`.
  `go.mod` is the floor, 1.26 like every Go module here
  (`docs/sdk/go-releasing.md`), proven by ci.yml's 1.26 leg.
- **`windows/arm` is gone.** Go no longer supports 32-bit Windows on ARM, and
  asking for it fails the whole cross-compile run rather than skipping.
- **bbolt takes an exclusive lock on the cache file.** A second invocation while
  a long `bulk` is running waits two seconds, warns, and continues without the
  cache. That is deliberate - a cache is an optimization and must never be the
  reason a lookup fails - but it does mean concurrent runs do not share it.
- **A command lives in FOUR places, and changing one drifts the rest silently.**
  The dispatch `switch` in `main.go`, the `subcommands` map beside it,
  `completions.go`, and the help text (plus the README). Nothing fails when they
  disagree: `printHelpWhoami` shipped `$ vpndetection --session work me` for two
  releases after `me` stopped being a command, and it answers "no addresses
  found in the input" rather than naming an unknown command. Same shape on the
  flag side - `login` grew `--paste` and `--no-browser` with the device flow
  and completion was never taught them. After any surface change, diff `--help`
  against `completions.go` against `subcommands`.
- **Check an advisory against a release binary, not against `go.mod`.** Most of
  the module graph never links - gin and echo arrive through
  `oapi-codegen/runtime` - so `go version -m` on a release binary is the set that
  ships. Releases are stripped, so whether a flagged symbol is actually linked
  takes `go tool nm` on an unstripped build for each affected GOOS.
- **Chocolatey has no 1.2.0 through 1.3.1.** Each push was refused (`403`) while
  1.1.0, the first version, sat in moderation; once it was approved
  (2026-09-24) only the latest tag, 1.3.2, went out. Every version still waits
  for a human reviewer: `docs/cli/channel-windows.md`.
