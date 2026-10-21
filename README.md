# dns-bench

**dns-bench** is a small, original command-line lab for measuring DNS **resolver** performance *from your network*. It times well-known public resolvers (Cloudflare, Google, Quad9, OpenDNS, NextDNS, AdGuard, and others), your OS-configured resolver, and any extra IPv4 addresses you pass in.

It prints a ranked table and can write JSON, CSV, or HTML reports. v1 is IPv4/UDP port 53 only.

## Disclaimer

dns-bench is an independent MIT-licensed tool. It is not affiliated with, endorsed by, or connected to Gibson Research Corporation (GRC), and it does not use GRC branding, logos, or software.

“DNS Benchmark” is a GRC product name. This project is dns-bench.

## Install

### Windows

Download `dns-bench-*-windows-amd64.zip` from [Releases](https://github.com/78tacos/dns-bench/releases) (`windows-arm64` on Windows on ARM). Unzip and run `dns-bench.exe`.

The Windows build reads the OS-configured DNS servers. Add more with `-resolver` or `-resolvers`. Allow UDP/53 if Windows Firewall asks.

Linux and macOS archives are on the same page (`*.tar.gz`).

### From source

Requires [Go](https://go.dev/) 1.22 or newer.

```bash
go test ./...
go build -o dns-bench ./cmd/dns-bench
```

On Windows that binary is `dns-bench.exe`.

```bash
go install github.com/78tacos/dns-bench/cmd/dns-bench@latest
```

## Run

A live run needs UDP/53 to the resolvers you probe. The unit tests do not; they use fake timings.

```bash
# system resolver plus the built-in public list
./dns-bench

# more samples, a DNSSEC probe, and reports
./dns-bench -queries 20 -dnssec -json report.json -csv report.csv -html report.html

# one LAN resolver and one public IP
./dns-bench -no-public -resolver 192.168.1.1 -resolver 1.1.1.1

# rank by cold QNAME or .com TLD-path latency
./dns-bench -rank uncached
./dns-bench -rank tld
```

| Flag | Meaning |
| --- | --- |
| `-queries N` | Timed queries per resolver per latency phase (default 8) |
| `-timeout 2s` | Per-query timeout |
| `-rank blended\|cached\|uncached\|tld\|reliability` | Sort order |
| `-resolver IP` | Extra IPv4, repeatable; optional `:port` |
| `-resolvers a,b` | Comma-separated extras |
| `-no-system` / `-no-public` | Skip the OS resolver or the built-in list |
| `-nxdomain` | NXDOMAIN rewrite probe (on by default; `-nxdomain=false` to skip) |
| `-tld` | .com TLD-path timing (on by default; `-tld=false` to skip) |
| `-dnssec` | DNSSEC-awareness probe |
| `-list` | Print the probe list and exit |
| `-json` / `-csv` / `-html` | Write a report |
| `-quiet` | Skip the banner and progress on stderr |

## How ranking works

Each resolver can run three latency phases, plus two optional checks.

**Uncached.** Query `u-<run>-<i>.<popular-domain>`. The label is unique, so the resolver has to recurse. A well-formed reply counts, including NXDOMAIN.

**Cached.** Query a popular name once to warm the cache, then time a second A query of the same name. Success is `NOERROR` with at least one A record. The warmup query is not scored.

**TLD-path (.com).** Query `c-<run>-<i>.com`, a unique name that does not exist. The resolver has to ask the .com servers. NXDOMAIN is a successful reply. An A record counts as NXDOMAIN rewrite.

**Loss.** Successful timed replies divided by timed attempts. Timeouts and malformed packets are failures. Min, avg, max, p50, p95, and stddev use successful samples only.

**NXDOMAIN rewrite** (on by default) queries `nx-<run>.invalid`. An A record instead of NXDOMAIN means the resolver intercepted the name. It is not a DNSSEC check.

**DNSSEC** (`-dnssec`) queries `dnssec-failed.org`. `SERVFAIL` means the resolver appears to validate; any other reply means it does not. This is a coarse flag, not a full audit.

The default **blended** score (higher is better) weights cached and uncached equally:

```
score = reliability / (0.5 * cached_p50_ms + 0.5 * uncached_p50_ms)
```

NXDOMAIN rewrite multiplies the score by 0.85. Other modes:

- `cached`, `uncached`, and `tld` use that phase’s p50 in the same formula
- `reliability` is the raw reply ratio, with blended p50 as the tie-break

Resolvers with no successful replies rank last.

Public anycast caches often already hold popular names, so a “cached” time on 1.1.1.1 or 8.8.8.8 is usually a hit at the resolver. The unique uncached labels and the TLD-path names show more of the recursive work.

## Tests

```bash
go test ./...
```

Tests cover packet encode/decode, stats, ranking, resolver parsing, report encoding, and a mocked bench. The query client is tested against a localhost UDP server. No internet access is required.

To try a real path after building:

```bash
./dns-bench -queries 3 -resolver 1.1.1.1 -no-public -no-system
```

## Scope

v1 includes IPv4 UDP/53, a public resolver list, the system resolver (Linux and macOS `/etc/resolv.conf`, Windows `GetNetworkParams`), custom IPs, cached / uncached / TLD-path latency, loss, NXDOMAIN rewrite, an optional DNSSEC flag, a ranked table, and JSON, CSV, and HTML reports.

Not in v1: IPv6, DoH, DoT, rebinding checks, and signed-domain auth timing.

## License

MIT © 2026 78tacos. See [LICENSE](LICENSE).
