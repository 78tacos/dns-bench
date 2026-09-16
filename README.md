# dns-bench

**dns-bench** is a small, original command-line lab for measuring DNS **resolver** performance *from your network*. It times well-known public resolvers (Cloudflare, Google, Quad9, OpenDNS, and others), your OS-configured resolver, and any extra IPv4 addresses you pass in.

It is a portfolio tool: ranked terminal output plus optional JSON / CSV / HTML reports. v1 is IPv4/UDP port 53 only.

## Disclaimer

dns-bench is an **independent MIT-licensed tool**. It is **not affiliated with, endorsed by, or connected to Gibson Research Corporation (GRC)**. It does not use GRC branding, logos, UI chrome, data files, or software, and it does not claim to be GRC-compatible.

“DNS Benchmark” is a GRC product name. This project is **dns-bench**.

Measurement *ideas* (cached vs uncached latency, TLD-path timing, reply reliability, NXDOMAIN rewrite, DNSSEC awareness) are widely discussed in public DNS literature. The implementation, scoring, copy, and presentation here are original.

## Install

### Windows

Download `dns-bench-*-windows-amd64.zip` from [Releases](https://github.com/78tacos/dns-bench/releases) (use `windows-arm64` on Windows on ARM). Unzip and run `dns-bench.exe`.

The Windows build reads the OS-configured DNS servers automatically. Add extras with `-resolver` / `-resolvers`. Allow UDP/53 if Windows Firewall prompts.

Linux and macOS archives are on the same release page (`*.tar.gz`).

### From source

Needs [Go](https://go.dev/) 1.22+.

From a clone of this repository:

```bash
go test ./...
go build -o dns-bench ./cmd/dns-bench
```

On Windows, `go build` produces `dns-bench.exe`.

```bash
go install github.com/78tacos/dns-bench/cmd/dns-bench@latest
```

## Run a live bench

A live run **needs network access** to UDP/53 on the resolvers you probe. Unit tests do **not**; they mock timings and ranking.

```bash
# system resolver + built-in public IPv4 list
./dns-bench

# more samples, DNSSEC probe, reports
./dns-bench -queries 20 -dnssec -json report.json -csv report.csv -html report.html

# only your LAN resolver plus one public IP
./dns-bench -no-public -resolver 192.168.1.1 -resolver 1.1.1.1

# rank by uncached (cold QNAME) or .com TLD-path latency
./dns-bench -rank uncached
./dns-bench -rank tld
```

Useful flags:

| Flag | Meaning |
| --- | --- |
| `-queries N` | Timed queries per resolver **per latency phase** (default 8) |
| `-timeout 2s` | Per-query timeout |
| `-rank blended\|cached\|uncached\|tld\|reliability` | How the table is sorted |
| `-resolver IP` | Extra IPv4 (repeatable; optional `:port`) |
| `-resolvers a,b` | Comma-separated extras |
| `-no-system` / `-no-public` | Drop OS resolver or the built-in list |
| `-nxdomain` | NXDOMAIN rewrite probe (default on; `-nxdomain=false` to skip) |
| `-tld` | .com TLD-path timing (default on; `-tld=false` to skip) |
| `-dnssec` | Optional DNSSEC-awareness probe |
| `-list` | Print the probe list and exit |
| `-json` / `-csv` / `-html` | Write reports |
| `-quiet` | No banner/progress on stderr |

## How ranking works

Each resolver can run three latency phases plus two optional one-shot checks:

1. **Uncached** — query `u-<run>-<i>.<popular-domain>`. The label is unique, so the resolver cannot answer that QNAME from cache and must recurse. A well-formed reply counts as success, **including NXDOMAIN**.
2. **Cached** — query a popular name once to warm the cache, then time a **second** A query of the same name. Success requires `NOERROR` plus at least one A record.
3. **TLD-path (.com)** — query `c-<run>-<i>.com`, a unique nonexistent second-level name. The resolver must consult .com TLD servers. NXDOMAIN is a successful reply; an A record is treated as NXDOMAIN rewrite.

**Reliability / loss %** = successful timed replies / timed attempts (warmup queries are not scored). Timeouts and malformed packets are failures. Latency stats (min / avg / max / p50 / p95 / stddev) use successful samples only.

**NXDOMAIN rewrite** (on by default) queries `nx-<run>.invalid`. An A record instead of NXDOMAIN is search/assist-style interception, not a DNSSEC spoof analysis.

**DNSSEC** (`-dnssec`) queries `dnssec-failed.org`. `SERVFAIL` → resolver appears to validate; any other reply → not validating. This is a coarse awareness flag, not a full DNSSEC audit.

Default **blended** score (higher is better) weights cached and uncached equally:

```
score = reliability / (0.5 * cached_p50_ms + 0.5 * uncached_p50_ms)
```

NXDOMAIN rewrite multiplies score by **0.85**. Other modes:

- `cached` / `uncached` / `tld` — same formula using only that phase’s p50
- `reliability` — raw reply ratio (ties broken by blended p50)

Resolvers with zero successful replies rank last.

Public anycast caches often already hold popular names, so “cached” on 1.1.1.1 / 8.8.8.8 is typically a cache hit at the resolver, not at your stub. Unique uncached labels and the TLD-path names are the better picture of recursive work.

## Tests

```bash
go test ./...
```

These cover packet encode/decode, stats, ranking, resolver parsing, report encoding, and a mocked bench engine. A localhost UDP fake server is used for the query client; nothing on the internet is required.

To exercise a real network path after building:

```bash
./dns-bench -queries 3 -resolver 1.1.1.1 -no-public -no-system
```

## Scope (v1)

Included: IPv4 UDP/53, curated public list, system resolver (Linux/macOS `/etc/resolv.conf`; Windows `GetNetworkParams`), custom IPs, cached / uncached / TLD-path latency, loss %, NXDOMAIN rewrite, optional DNSSEC flag, ranked table, JSON/CSV/HTML.

Follow-ups (not in v1): IPv6, DoH, DoT, rebinding checks, signed-domain auth timing.

## License

MIT © 2026 78tacos — see [LICENSE](LICENSE).
