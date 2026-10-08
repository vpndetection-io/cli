# Changelog

What each release changed for you, newest first. Each line is a commit's summary, linked to its full description and diff. Releases before 1.3.2 are described by the commits between their tags.

## 1.5.0 - 2026-10-08

### Breaking changes

- Print help for a bare session, as for every other command group ([`087dd4c`](https://github.com/vpndetection-io/cli/commit/087dd4c229fb1188c6b620bfb4dd3d5c0966ae1c))

## 1.4.0 - 2026-10-07

### Fixes

- Exit 1 when an address could not be answered ([`cccb4a2`](https://github.com/vpndetection-io/cli/commit/cccb4a235a7ca9971160538596a0a5a92b396c25))

## 1.3.6 - 2026-10-06

### Fixes

- Take libgo-iputil v1.1.2: bulk no longer prompts when stdin is /dev/null ([`a072ed5`](https://github.com/vpndetection-io/cli/commit/a072ed5da4aca5aeaaae4fe0ecdf3c23db4dec3c))

## 1.3.5 - 2026-10-05

### Fixes

- Refuse to prompt for a key when stdin is /dev/null ([`fdaa4b3`](https://github.com/vpndetection-io/cli/commit/fdaa4b3591b09f4d19d951abe22427087d06d0ae))

## 1.3.4 - 2026-09-29

### Fixes

- Take sdk-go v5.4.3: answer 26 more reserved ranges as bogons ([`60961f4`](https://github.com/vpndetection-io/cli/commit/60961f4886a09a7d7594a48a23cdd6c627bb180d))

## 1.3.3 - 2026-09-28

### Fixes

- Take sdk-go v5.4.2: bound server-set waits, look up ::ffff: CIDRs ([`eb1cac2`](https://github.com/vpndetection-io/cli/commit/eb1cac251bb6225b5d4f4dc3585a4d2f44ca87a5))

## 1.3.2 - 2026-09-23

### Fixes

- Print db list's LICENSE header the US way ([`b37a83c`](https://github.com/vpndetection-io/cli/commit/b37a83c7ef1bb2cf9dba05b1f30a9b22ce5f230e))
