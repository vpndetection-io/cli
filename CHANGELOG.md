# Changelog

What each release changed for you, newest first. Each line is a commit's summary, linked to its full description and diff. Releases before 1.3.2 are described by the commits between their tags.

## 1.11.2 - 2026-10-29

### Fixes

- Take sdk-go v5.10.2: read a lowercase t or z in a date-time ([`b6fe8ee`](https://github.com/vpndetection-io/cli/commit/b6fe8eea002d793bd48e58ab599e2aebdee8d09c))

## 1.11.1 - 2026-10-22

### Fixes

- Take sdk-go v5.10.0: read dates only in the zones they allow ([`8a9a663`](https://github.com/vpndetection-io/cli/commit/8a9a663d4fede294ac63c3696ceb8d34f408ed55))

## 1.11.0 - 2026-10-20

### Features

- Add the location and asn columns, and leave vpn.method out of the block ([`a0cf158`](https://github.com/vpndetection-io/cli/commit/a0cf158427ac5dcebcdf75847d966d2e3f7b1a53))

## 1.10.1 - 2026-10-20

### Fixes

- Build on Go 1.27.3, which distrusts a handful of intermediates ([`0755697`](https://github.com/vpndetection-io/cli/commit/07556979a7b857e275ee0872be0abef449b75035))

## 1.10.0 - 2026-10-19

### Breaking changes

- Take sdk-go v5.8.0, and print is_pubproxy and the pubproxy object ([`f949a13`](https://github.com/vpndetection-io/cli/commit/f949a1352ab371ee038839ad709c604e5712749b))

### Fixes

- Take libgo-iputil v1.1.3: keep a list's first address after a byte order mark ([`a1ba0dc`](https://github.com/vpndetection-io/cli/commit/a1ba0dc43c6b055c444d910ce2dd0efaa0407a3d))

## 1.9.1 - 2026-10-17

### Fixes

- Show a license that has not started yet by its start date, not as lapsed ([`b5d53e7`](https://github.com/vpndetection-io/cli/commit/b5d53e72aa2439d7c6386536d11eb07258616de1))

## 1.9.0 - 2026-10-16

### Breaking changes

- Refuse an argument that is no address and names no file ([`ac99e37`](https://github.com/vpndetection-io/cli/commit/ac99e379a739a080734e5c2f7960b7d67cd854f3))

## 1.8.1 - 2026-10-16

### Fixes

- Take sdk-go v5.5.3: retry an unreadable 2xx, refuse control chars in a key ([`508f7c0`](https://github.com/vpndetection-io/cli/commit/508f7c09c7775f63bc24e5ef819a201b45a14a71))

## 1.8.0 - 2026-10-14

### Breaking changes

- Append hits_days_pct and providers_num to CSV, and take them in --field ([`a5fe255`](https://github.com/vpndetection-io/cli/commit/a5fe255705bfa6be3111178b1ee58dde44fd9940))

### Fixes

- Print an empty detail object as {} under --show-absent and --field ([`def1010`](https://github.com/vpndetection-io/cli/commit/def1010bc83afacfcff35ca17ab9eb02dfbe1875))

## 1.7.0 - 2026-10-13

### Breaking changes

- Apply --field to JSON, JSON Lines and YAML, as to CSV ([`847880e`](https://github.com/vpndetection-io/cli/commit/847880ef4eb9b06e70c003ac58f6970b2d8c3db1))
- Print YAML with the API's field names and nesting, as JSON does ([`5d27b4e`](https://github.com/vpndetection-io/cli/commit/5d27b4efaa783b70bb2215056e01d248f5d3b6fc))

### Fixes

- Print an empty provider as "", not as the {} of a false flag ([`116b58e`](https://github.com/vpndetection-io/cli/commit/116b58ea1261b6283359dc036b7f4386ed54430b))

## 1.6.2 - 2026-10-12

### Fixes

- Take libgo-complete v1.0.2: offer options again after a boolean flag ([`b3a6d13`](https://github.com/vpndetection-io/cli/commit/b3a6d1333c3d636830a0ce01ececf8bda95931f7))

## 1.6.1 - 2026-10-11

### Fixes

- Say what --show-absent does, now that nothing is listed at the end ([`2952603`](https://github.com/vpndetection-io/cli/commit/29526033d72dd862a70f6c154221f88901a3dee9))
- Complete --retries for database's subcommands ([`0934109`](https://github.com/vpndetection-io/cli/commit/0934109da91e1713b0bd190a010a567f68040caf))

## 1.6.0 - 2026-10-10

### Breaking changes

- Refuse a config file other accounts can open, as ssh does ([`c3291af`](https://github.com/vpndetection-io/cli/commit/c3291af067d208ef6209f93dfbb6eab4a93504f1))

### Fixes

- Build on Go 1.27.2, which fixes eight standard-library advisories ([`287fe3c`](https://github.com/vpndetection-io/cli/commit/287fe3c135988d1c5f9815b7469113e9347b3919))
- Never save over a config that could not be read ([`0b31fd4`](https://github.com/vpndetection-io/cli/commit/0b31fd4d976b95c0cb1b09106bf147345653fb64))
- Trim a key given by --key or the environment, as login does ([`1846ae8`](https://github.com/vpndetection-io/cli/commit/1846ae81d8a0d19db476b57ea8cb7b91a513d04b))
- Fail whoami --json with no key, rather than print prose ([`44f1ff5`](https://github.com/vpndetection-io/cli/commit/44f1ff58273abc3ed500161746874b3b80c8365c))
- Point CI at VPNDETECTION_API_KEY, off the command line ([`3afd319`](https://github.com/vpndetection-io/cli/commit/3afd319742b4e64da3af802882dd7b17f0e55a25))
- List --retries in database's help ([`acf5ea5`](https://github.com/vpndetection-io/cli/commit/acf5ea5d931c8bb341bac4451a972a5dde9c5fd0))
- Take sdk-go v5.5.2: read a Retry-After as digits or a date only ([`6663d37`](https://github.com/vpndetection-io/cli/commit/6663d37aeb86bc6792ab2ff65266484e47c26b58))

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
