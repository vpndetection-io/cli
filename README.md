# [<img src="https://s3.vpndetection.io/vpndetection-public/brand/icon.svg" width="24"/>](https://vpndetection.io/) VPNDetection CLI

Look up what an IP address is — a VPN exit, a hosting or CDN range, a Tor node, a privacy relay, or a residential, datacenter or mobile proxy — one address at a time or a few million, and download the licensed datasets.

```console
$ vpndetection 45.83.91.1
ip        45.83.91.1
is_bogon  false
is_vpn    true
```

No account needed: the free tier answers `ip` and `is_vpn`. A key widens the answer and raises the allowance.

## Getting Started

**macOS**

```bash
brew tap vpndetection-io/tap && brew install vpndetection
```

**Debian / Ubuntu** — with updates through apt:

```bash
echo "deb [trusted=yes] https://s3.vpndetection.io/vpndetection-public/apt/ /" | sudo tee /etc/apt/sources.list.d/vpndetection.list
sudo apt update && sudo apt install vpndetection
```

**Windows**

```powershell
winget install Mslm.VPNDetection
# or
choco install vpndetection
```

**Anywhere else** — binaries for 22 platform/architecture pairs are on the [releases page](https://github.com/vpndetection-io/cli/releases), or:

```bash
curl -Ls https://github.com/vpndetection-io/cli/releases/latest/download/macos.sh | sh   # macOS
curl -Ls https://github.com/vpndetection-io/cli/releases/latest/download/deb.sh | sh     # one-off .deb
docker run --rm ghcr.io/vpndetection-io/cli 45.83.91.1                                   # container
```

Then turn on shell completion, which knows the commands, the flags, the field names and your own session names:

```bash
vpndetection completion install
```

## Looking things up

Addresses, CIDRs, ranges and files are accepted in any mix, from arguments and standard input at the same time.

```bash
vpndetection 1.1.1.1 8.8.8.0/24 9.9.9.1-9.9.9.9 addresses.txt
cat addresses.txt | vpndetection
vpndetection 10.0.0.0/16 --csv > answers.csv
```

One address prints the readable block; anything resolving to more than one prints JSON keyed by address. That is decided by how many addresses the input **resolves to**, so `10.0.0.1/32` is one and `10.0.0.0/30` is four. `vpndetection bulk` forces the machine format whatever the count, which is what a script wants.

Output is `--pretty`, `--json`, `--jsonl`, `--csv` or `--yaml`, and `-f` selects fields:

```bash
vpndetection 45.83.91.1 -f is_vpn,vpn.provider
vpndetection 45.83.91.1 -f vpn                 # the whole object
```

Bulk runs stream. Nothing is held in memory beyond one batch, so a `/8` is a long command rather than an impossible one, and output starts immediately.

Private, reserved and documentation addresses are answered locally without a request, so they cost nothing and work offline.

## Absent is not false

**A field your plan does not include is missing from the answer, not `false`.** `is_tor` absent means we did not check; `is_tor: false` means we checked and it is not a Tor node. The pretty output prints only what you were served and names the rest at the end; CSV writes an empty cell for absent and the literal `false` for false. Use `--show-absent` to see them all.

## Keys and sessions

```bash
vpndetection login                    # prompts, does not echo, does not reach your shell history
vpndetection whoami
```

Credentials are stored in **named sessions**, so one machine can hold several organizations' keys, or staging alongside production:

```bash
vpndetection login --session work
vpndetection login --session staging --base-url https://api-staging.vpndetection.io
vpndetection session list
vpndetection session use work
vpndetection --session staging 1.1.1.1   # one command, without switching
```

A key can also come from `--key` or `VPNDETECTION_API_KEY`, in that order of precedence. The config file is written `0600` in a `0700` directory.

## Databases

The same classifications, published as files you host yourself. Access is granted by contract and needs a key carrying the `db.download` scope.

```bash
vpndetection db list
vpndetection db metadata vpn_ip_v1
vpndetection db download vpn_ip_v1                 # verifies sha256
vpndetection db download cdn_ip_v1 --format mmdb
curl -fL "$(vpndetection db url vpn_ip_v1)" -o vpn_ip_v1.csv.gz
```

Downloads are verified against the published checksum and land through a `.part` file, so an interrupted transfer never leaves a truncated file that reads as a whole dataset.

## Cache

Answers are cached on disk, so looking the same address up twice costs one request. Entries are partitioned by **credential**, because which fields an answer carries depends on the plan behind the key — one key's answers are never served to another's.

```bash
vpndetection cache info
vpndetection cache clear
vpndetection --nocache 1.1.1.1
vpndetection config cache=disable cache_ttl=24h
```

## Other Libraries

There are official [VPNDetection client libraries](https://vpndetection.io/docs/integrations) for Node.js, Python, Go, Java, .NET, PHP, Ruby, Rust, Swift, Erlang, Perl and Zig, plus an [MCP server](https://vpndetection.io/docs/integrations/mcp) for AI agents.

## About VPNDetection

[VPNDetection](https://vpndetection.io) answers what an IP address is: VPN infrastructure, hosting and CDN ranges, Tor nodes, privacy relays, and residential, datacenter and mobile proxy pools. Query it per address through the API, or license the datasets and host them yourself.

## Licence

MIT.
