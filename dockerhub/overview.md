# VPNDetection CLI

The official command line for the [VPNDetection](https://vpndetection.io) API. It looks up what an IP address is (a VPN exit, a hosting or CDN range, a Tor node, a privacy relay, or a residential, datacenter or mobile proxy), one address at a time or a few million.

The full guide is at [docs.vpndetection.io/integrations/cli](https://docs.vpndetection.io/integrations/cli), and the source at [github.com/vpndetection-io/cli](https://github.com/vpndetection-io/cli).

## Usage

```bash
docker run --rm vpndetection/cli 146.70.116.130
```

No key is needed, since the free plan answers any address. Pass yours from the environment for every field your plan includes:

```bash
docker run --rm -e VPNDETECTION_API_KEY vpndetection/cli 146.70.116.130
```

A list on stdin needs `-i`:

```bash
docker run --rm -i -e VPNDETECTION_API_KEY vpndetection/cli bulk --csv < addresses.txt > answers.csv
```

## Tags

`latest` and each release's version, for `linux/amd64` and `linux/arm64`. The same image is published as `ghcr.io/vpndetection-io/cli`.
