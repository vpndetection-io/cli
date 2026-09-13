# [<img src="https://s3.vpndetection.io/vpndetection-public/brand/mark.svg" alt="VPNDetection" width="24"/>](https://vpndetection.io/) VPNDetection CLI

[![release](https://img.shields.io/github/v/release/vpndetection-io/cli)](https://github.com/vpndetection-io/cli/releases)
[![license](https://img.shields.io/github/license/vpndetection-io/cli)](LICENSE)

The official command line interface for the [VPNDetection](https://vpndetection.io) API.

Look up what an IP address is — a VPN exit, a hosting or CDN range, a Tor node, a privacy relay, or a residential, datacenter or mobile proxy — one address at a time or a few million, and download the licensed databases.

## Getting Started

No account is needed to start: the free tier answers `ip` and `is_vpn`, and allows 1000 requests per day per source address.

### macOS

```bash
brew tap vpndetection-io/tap
brew install vpndetection
```

### Debian / Ubuntu

Install from our apt repository, which keeps the CLI up to date with `apt upgrade`:

```bash
echo "deb [trusted=yes] https://apt.vpndetection.io/ /" | sudo tee /etc/apt/sources.list.d/vpndetection.list
sudo apt update && sudo apt install vpndetection
```

Or install a single `.deb` without the repository:

```bash
curl -Ls https://github.com/vpndetection-io/cli/releases/latest/download/deb.sh | sh
```

### Windows

```powershell
winget install Mslm.VPNDetection
```

```powershell
choco install vpndetection
```

Or install for the current user without a package manager:

```powershell
iwr -useb https://github.com/vpndetection-io/cli/releases/latest/download/windows.ps1 | iex
```

### Docker

```bash
docker run --rm ghcr.io/vpndetection-io/cli 45.83.91.1
```

### Using `go install`

```bash
go install github.com/vpndetection-io/cli@latest
```

The binary is named after the module, so rename it to `vpndetection` if you want the examples below to read the same.

### Using `curl` / `wget`

Binaries are published for 22 platform and architecture pairs on the [releases page](https://github.com/vpndetection-io/cli/releases). Pick yours:

```bash
# Linux amd64; for Windows use ".zip" instead of ".tar.gz"
curl -LO https://github.com/vpndetection-io/cli/releases/latest/download/vpndetection_0.1.0_linux_amd64.tar.gz
tar -xzf vpndetection_0.1.0_linux_amd64.tar.gz
sudo mv vpndetection /usr/local/bin/
```

macOS has a one-line installer that picks the right architecture for you:

```bash
curl -Ls https://github.com/vpndetection-io/cli/releases/latest/download/macos.sh | sh
```

Note that the binaries are not code-signed, so macOS Gatekeeper will ask before running one the first time, and Windows SmartScreen may warn.

### From source

```bash
git clone https://github.com/vpndetection-io/cli
cd cli
./scripts/build.sh
```

The result lands in `build/`. Go 1.25 or newer.

## Quick Start

### Default help message

Invoking the CLI with nothing shows what it can do:

```console
$ vpndetection
Usage: vpndetection <ip | cidr | range | file>... [<opts>]
       vpndetection <cmd> [<opts>] [<args>]
...
```

### Login

You can use the CLI without an account, but a key widens the answer and raises your allowance. Create one in the [console](https://app.vpndetection.io), then:

```console
$ vpndetection login
API key:
stored key mk_1************************abcd in session "default", now active
```

The prompt does not echo and the key does not reach your shell history. `vpndetection signup` opens the console to create an account first.

### My IP

```console
$ vpndetection myip
ip        45.83.91.1
is_bogon  false
is_vpn    true
```

This is the address our edge observed, so behind a proxy or a VPN it reports the exit you left through.

### Any IP

```console
$ vpndetection 45.83.91.1
ip                  45.83.91.1
is_bogon            false
is_vpn              true
is_hosting          true
is_relay            false
is_tor              false
is_cdn              false
vpn.provider        mullvad
vpn.last_seen       2026-09-12
hosting.provider    m247
hosting.confidence  high
```

Private, reserved and documentation addresses are answered locally without a request, so they cost nothing and work offline.

### Piping

Addresses can come from standard input, and from arguments, at the same time:

```console
$ cat addresses.txt | vpndetection
$ vpndetection 1.1.1.1 8.8.8.0/24 9.9.9.1-9.9.9.9 addresses.txt
```

Anything resolving to more than one address prints JSON keyed by address; a single address prints the readable block. That follows the resolved **count**, so `10.0.0.1/32` is one and `10.0.0.0/30` is four.

### Field filter

```console
$ vpndetection 45.83.91.1 -f is_vpn
is_vpn  true

$ vpndetection 45.83.91.1 -f is_vpn,vpn.provider
is_vpn        true
vpn.provider  mullvad

$ vpndetection 45.83.91.1 -f vpn          # the whole object
```

### Bulk

`bulk` always prints the machine format, whatever the count, which is what a script wants:

```console
$ vpndetection bulk 8.8.8.0/24 --csv > answers.csv
$ vpndetection bulk addresses.txt --jsonl | jq -r 'select(.is_vpn) | .ip'
```

Bulk runs stream: nothing is held in memory beyond one batch, so a `/8` is a long command rather than an impossible one, and output starts immediately. With no arguments at a terminal it reads addresses you type, one per line, ending at a blank line.

Output formats are `--pretty`, `--json`, `--jsonl`, `--csv` and `--yaml`.

### Sessions

Credentials are stored in named sessions, so one machine can hold several organizations' keys, or staging alongside production:

```console
$ vpndetection login --session work
$ vpndetection login --session staging --base-url https://api-staging.vpndetection.io
$ vpndetection session list
   NAME     KEY                       API                                   LAST USED
*  default  mk_1***************abcd   default                               2h ago
   work     mk_9***************wxyz   default                               never
   staging  mk_4***************mnop   https://api-staging.vpndetection.io   never

$ vpndetection session use work
$ vpndetection --session staging 45.83.91.1    # one command, without switching
```

A key can also come from `--key` or the `VPNDETECTION_API_KEY` environment variable, in that order of precedence. The config file is written `0600` inside a `0700` directory.

### Databases

The same classifications, published as files you host yourself. Access is granted by contract and needs a key carrying the `db.download` scope.

```console
$ vpndetection db list
ID          NAME     LICENCE   STANDING   TERM
vpn_ip_v1   VPN IP   standard  licensed   renews 2027-01-04

$ vpndetection db metadata vpn_ip_v1
$ vpndetection db download vpn_ip_v1
$ vpndetection db download cdn_ip_v1 --format mmdb cdn.mmdb
$ curl -fL "$(vpndetection db url vpn_ip_v1)" -o vpn_ip_v1.csv.gz
```

Downloads are verified against the published sha256 and land through a `.part` file, so an interrupted transfer never leaves a truncated file that reads as a whole database.

## Auto-Completion

Auto-completion is supported for at least the following shells:

```
bash
zsh
fish
```

It completes commands, flags, the values flags take, field names, and the session names this machine actually holds.

NOTE: it may work for other shells as well, because the implementation is in Go and is not shell-specific.

### Installation

Installing auto-completions is as simple as running one command, which covers `bash`, `zsh` and `fish`:

```bash
vpndetection completion install
```

Start a new shell afterwards to pick it up.

If you want to customize the installation (for example when the auto-installation does not work as expected), you can request the completion line for each shell instead:

```bash
# get bash completion script
vpndetection completion bash

# get zsh completion script
vpndetection completion zsh

# get fish completion script
vpndetection completion fish
```

`vpndetection completion uninstall` removes it again.

### Shell not listed?

If your shell is not listed here, you can open an issue.

Note that as long as the `COMP_LINE` environment variable is provided to the binary itself, it will output completion results. So if your shell provides a way to pass `COMP_LINE` on auto-completion attempts to a binary, then have your shell do that with the `vpndetection` binary itself.

## Data

How much you get back per lookup depends on the plan behind your key. See the [plans](https://vpndetection.io/pricing) for what each includes. All examples in this document use a key with everything enabled.

**A field your plan does not include is missing from the answer, not `false`.** `is_tor` absent means we did not check; `is_tor: false` means we checked and it is not a Tor node. The readable output prints only what you were served and names the rest at the end; CSV writes an empty cell for absent and the literal `false` for false. `--show-absent` renders them all.

## Caching

Answers are cached on disk, so looking the same address up twice costs one request rather than two.

```bash
vpndetection cache info
vpndetection cache clear
vpndetection --nocache 45.83.91.1
vpndetection config cache=disable cache_ttl=24h
```

Entries are partitioned by **credential**, because which fields an answer carries depends on the plan behind the key — one key's answers are never served to another's. Clearing empties every partition.

## Color Output

### Disabling color output

The CLI respects either the `--nocolor` flag or the [`NO_COLOR`](https://no-color.org/) environment variable to disable color output.

### Color on Windows

To enable color support for the Windows command prompt, run the following to enable [`Console Virtual Terminal Sequences`](https://docs.microsoft.com/en-us/windows/console/console-virtual-terminal-sequences):

```cmd
REG ADD HKCU\CONSOLE /f /v VirtualTerminalLevel /t REG_DWORD /d 1
```

You can disable this by running the following:

```cmd
REG DELETE HKCU\CONSOLE /f /v VirtualTerminalLevel
```

## Other Libraries

There are official VPNDetection client libraries available for many languages including PHP, Python, Go, Java, Ruby, and many popular frameworks such as Django, Rails, and Laravel. See our GitHub at https://github.com/vpndetection-io for more.

## About VPNDetection

VPN Detection API: Accurate anonymity detection identifying VPNs, residential proxies, hosting servers, Tor nodes, CDNs, relays and more.

[<img src="https://s3.vpndetection.io/vpndetection-public/brand/mark.svg" alt="VPNDetection" width="96"/>](https://vpndetection.io/)

## License

This project is licensed under the [MIT License](LICENSE).
