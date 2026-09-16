# dns-bench

**dns-bench** is a small, original command-line lab for measuring DNS **resolver** performance *from your network*. It times well-known public resolvers (Cloudflare, Google, Quad9, OpenDNS, and others), your OS-configured resolver, and any extra IPv4 addresses you pass in.

It is a portfolio tool: ranked terminal output plus optional JSON / CSV / HTML reports. v1 is IPv4/UDP port 53 only.

## Disclaimer

dns-bench is an **independent MIT-licensed tool**. It is **not affiliated with, endorsed by, or connected to Gibson Research Corporation (GRC)**. It does not use GRC branding, logos, UI chrome, data files, or software, and it does not claim to be GRC-compatible.

“DNS Benchmark” is a GRC product name. This project is **dns-bench**.

Measurement *ideas* (cached vs uncached latency, reply reliability, NXDOMAIN rewrite as a flag) are widely discussed in public DNS literature. The implementation, scoring, copy, and presentation here are original.

## Install

Needs [Go](https://go.dev/) 1.22+.

From a clone of this repository:

```bash
go test ./...
go build -o dns-bench ./cmd/dns-bench
```

Install the module (after this code is on the branch or tag you want):

```bash
go install github.com/78tacos/dns-bench/cmd/dns-bench@cursor/feat-v1-dns-bench-10d1
```

`go install …@latest` tracks the module’s default branch once a release is published there.

## Run a live bench

A live run **needs network access** to UDP/53 on the resolvers you probe. Unit tests do **not**; they mock timings and ranking.

```bash
# system resolver + built-in public IPv4 list
./dns-bench

# more samples, NXDOMAIN rewrite check, reports
./dns-bench -queries 20 -nxdomain -json report.json -csv report.csv -html report.html

# only your LAN resolver plus one public IP
./dns-bench -no-public -resolver 192.168.1.1 -resolver 1.1.1.1

# rank by uncached (cold QNAME) latency
./dns-bench -rank uncached
```

Useful flags:

| Flag | Meaning |
| --- | --- |
| `-queries N` | Timed queries per resolver **per phase** (default 8) |
| `-timeout 2s` | Per-query timeout |
| `-rank blended\|cached\|uncached\|reliability` | How the table is sorted |
| `-resolver IP` | Extra IPv4 (repeatable; optional `:port`) |
| `-resolvers a,b` | Comma-separated extras |
| `-no-system` / `-no-public` | Drop OS resolver or the built-in list |
| `-nxdomain` | Probe `.invalid` for rewrite-to-A behavior |
| `-list` | Print the probe list and exit |
| `-json` / `-csv` / `-html` | Write reports |
| `-quiet` | No banner/progress on stderr |

## How ranking works

Each resolver gets two phases:

1. **Uncached** — query `u-<run>-<i>.<popular-domain>`. The label is unique, so the resolver cannot answer that QNAME from cache and must recurse. A well-formed reply counts as success, **including NXDOMAIN**.
2. **Cached** — query a popular name once to warm the cache, then time a **second** A query of the same name. Success requires `NOERROR` plus at least one A record.

**Reliability** = successful timed replies / timed attempts (warmup queries are not scored). Timeouts and malformed packets are failures. Latency stats (min / avg / max / p50 / p95) use successful samples only.

Default **blended** score (higher is better):

```
score = reliability / (0.5 * cached_p50_ms + 0.5 * uncached_p50_ms)
```

If `-nxdomain` sees A records for a reserved `.invalid` name, the score is multiplied by **0.85**. Other modes:

- `cached` / `uncached` — same formula using only that phase’s p50
- `reliability` — raw reply ratio (ties broken by blended p50)

Resolvers with zero successful replies rank last.

Public anycast caches often already hold popular names, so “cached” on 1.1.1.1 / 8.8.8.8 is typically a cache hit at the resolver, not at your stub. Unique uncached labels are the better picture of recursive work.

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

Included: IPv4 UDP/53, curated public list, system resolver via `/etc/resolv.conf` (Linux/macOS; on Windows pass `-resolver`), custom IPs, ranked table, JSON/CSV/HTML, NXDOMAIN rewrite flag.

Follow-ups (not in v1): IPv6, DoH, DoT, TLD-path timing, DNSSEC, rebinding checks.

## License

MIT © 2026 78tacos — see [LICENSE](LICENSE).
