package report

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"strings"
	"time"

	"github.com/78tacos/dns-bench/internal/rank"
	"github.com/78tacos/dns-bench/internal/stats"
	"github.com/78tacos/dns-bench/internal/version"
)

// Disclaimer is shown in README, CLI, and exported reports.
const Disclaimer = "dns-bench is an independent MIT-licensed tool. It is not affiliated with, endorsed by, or connected to Gibson Research Corporation (GRC). It does not use GRC branding, data files, or software. “DNS Benchmark” is a GRC product name; this project is dns-bench."

// Table renders a ranked terminal table.
func Table(rows []rank.Row) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-4s  %-24s  %-16s  %-16s  %-16s  %-7s  %-8s  %-8s  %s\n",
		"RANK", "RESOLVER", "CACHED p50/p95", "UNCACHED p50/p95", "TLD p50/p95", "LOSS", "NX", "DNSSEC", "SCORE")
	fmt.Fprintf(&b, "%s\n", strings.Repeat("-", 128))
	if len(rows) == 0 {
		b.WriteString("(no results)\n")
		return b.String()
	}
	for i, r := range rows {
		nx := "—"
		if r.NXChecked {
			if r.NXRewrite {
				nx = "rewrite"
			} else {
				nx = "ok"
			}
		} else if r.NXRewrite {
			nx = "rewrite"
		}
		dnssec := "—"
		if r.DNSSECChecked {
			if r.DNSSECValidate {
				dnssec = "yes"
			} else {
				dnssec = "no"
			}
		}
		sys := ""
		if r.System {
			sys = "*"
		}
		fmt.Fprintf(&b, "%-4d  %-24s  %-16s  %-16s  %-16s  %-7s  %-8s  %-8s  %.3f\n",
			i+1,
			truncate(sys+r.Name+" "+r.Address, 24),
			pair(r.Cached),
			pair(r.Uncached),
			pair(r.TLD),
			fmt.Sprintf("%.1f%%", (1-r.Reliability)*100),
			nx,
			dnssec,
			r.Score,
		)
	}
	return b.String()
}

func pair(s stats.Summary) string {
	if s.Count == 0 {
		return "—"
	}
	return fmt.Sprintf("%s / %s ms", fmtMS(s.P50), fmtMS(s.P95))
}

func fmtMS(d time.Duration) string {
	return fmt.Sprintf("%.1f", stats.Milliseconds(d))
}

func dnssecLabel(r rank.Row) string {
	if !r.DNSSECChecked {
		return ""
	}
	if r.DNSSECValidate {
		return "validating"
	}
	return "no"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// SummaryLine is a short original blurb (not a third-party "Conclusions" text).
func SummaryLine(rows []rank.Row, mode rank.Mode) string {
	if len(rows) == 0 {
		return "No resolvers produced results."
	}
	top := rows[0]
	if top.Reliability <= 0 {
		return "Every resolver failed to answer; check UDP/53 connectivity."
	}
	line := fmt.Sprintf(
		"Top pick on this network (%s rank): %s %s — cached p50 %s ms, uncached p50 %s ms, TLD p50 %s ms, %.0f%% replies.",
		mode.String(),
		top.Name,
		top.Address,
		fmtMS(top.Cached.P50),
		fmtMS(top.Uncached.P50),
		fmtMS(top.TLD.P50),
		top.Reliability*100,
	)
	if top.NXRewrite {
		line += " NXDOMAIN rewrite detected (a reserved/nonexistent name returned an A record)."
	}
	return line
}

// FileReport is the JSON document written by -json.
type FileReport struct {
	Tool        string    `json:"tool"`
	Version     string    `json:"version"`
	GeneratedAt string    `json:"generated_at"`
	RankMode    string    `json:"rank_mode"`
	Disclaimer  string    `json:"disclaimer"`
	Methodology string    `json:"methodology"`
	Summary     string    `json:"summary"`
	Results     []jsonRow `json:"results"`
}

type jsonRow struct {
	Rank        int     `json:"rank"`
	Name        string  `json:"name"`
	Address     string  `json:"address"`
	System      bool    `json:"system"`
	CachedP50MS float64 `json:"cached_p50_ms"`
	CachedP95MS float64 `json:"cached_p95_ms"`
	CachedMinMS float64 `json:"cached_min_ms"`
	CachedAvgMS float64 `json:"cached_avg_ms"`
	CachedMaxMS float64 `json:"cached_max_ms"`
	UncachedP50 float64 `json:"uncached_p50_ms"`
	UncachedP95 float64 `json:"uncached_p95_ms"`
	UncachedMin float64 `json:"uncached_min_ms"`
	UncachedAvg float64 `json:"uncached_avg_ms"`
	UncachedMax float64 `json:"uncached_max_ms"`
	UncachedSD  float64 `json:"uncached_stddev_ms"`
	TLDP50      float64 `json:"tld_p50_ms"`
	TLDP95      float64 `json:"tld_p95_ms"`
	TLDMin      float64 `json:"tld_min_ms"`
	TLDAvg      float64 `json:"tld_avg_ms"`
	TLDMax      float64 `json:"tld_max_ms"`
	TLDSD       float64 `json:"tld_stddev_ms"`
	CachedSD    float64 `json:"cached_stddev_ms"`
	Reliability float64 `json:"reliability"`
	Successes   int     `json:"successes"`
	Attempts    int     `json:"attempts"`
	NXRewrite   bool    `json:"nx_rewrite"`
	NXChecked   bool    `json:"nx_checked"`
	DNSSEC      string  `json:"dnssec"`
	Score       float64 `json:"score"`
}

const methodology = "Cached latency is the timed A query after a warmup of the same popular name. Uncached latency uses a unique label (u-<run>-<i>.<domain>) so the QNAME is not already in cache; NXDOMAIN still counts as a successful reply. TLD-path latency queries a unique nonexistent .com SLD (c-<run>-<i>.com) so the resolver must consult .com TLD servers. Reliability is successful replies / timed attempts. Default rank is equal-weight blended cached+uncached p50 with reliability in the numerator and an 0.85 multiplier if NXDOMAIN rewrite is detected. Optional DNSSEC check queries dnssec-failed.org: SERVFAIL is treated as validating, any other reply as not validating."

// JSON encodes a report.
func JSON(rows []rank.Row, mode rank.Mode, now time.Time) ([]byte, error) {
	rep := FileReport{
		Tool:        "dns-bench",
		Version:     version.Version,
		GeneratedAt: now.UTC().Format(time.RFC3339),
		RankMode:    mode.String(),
		Disclaimer:  Disclaimer,
		Methodology: methodology,
		Summary:     SummaryLine(rows, mode),
		Results:     make([]jsonRow, 0, len(rows)),
	}
	for i, r := range rows {
		rep.Results = append(rep.Results, jsonRow{
			Rank:        i + 1,
			Name:        r.Name,
			Address:     r.Address,
			System:      r.System,
			CachedP50MS: stats.Milliseconds(r.Cached.P50),
			CachedP95MS: stats.Milliseconds(r.Cached.P95),
			CachedMinMS: stats.Milliseconds(r.Cached.Min),
			CachedAvgMS: stats.Milliseconds(r.Cached.Avg),
			CachedMaxMS: stats.Milliseconds(r.Cached.Max),
			CachedSD:    stats.Milliseconds(r.Cached.StdDev),
			UncachedP50: stats.Milliseconds(r.Uncached.P50),
			UncachedP95: stats.Milliseconds(r.Uncached.P95),
			UncachedMin: stats.Milliseconds(r.Uncached.Min),
			UncachedAvg: stats.Milliseconds(r.Uncached.Avg),
			UncachedMax: stats.Milliseconds(r.Uncached.Max),
			UncachedSD:  stats.Milliseconds(r.Uncached.StdDev),
			TLDP50:      stats.Milliseconds(r.TLD.P50),
			TLDP95:      stats.Milliseconds(r.TLD.P95),
			TLDMin:      stats.Milliseconds(r.TLD.Min),
			TLDAvg:      stats.Milliseconds(r.TLD.Avg),
			TLDMax:      stats.Milliseconds(r.TLD.Max),
			TLDSD:       stats.Milliseconds(r.TLD.StdDev),
			Reliability: r.Reliability,
			Successes:   r.Successes,
			Attempts:    r.Attempts,
			NXRewrite:   r.NXRewrite,
			NXChecked:   r.NXChecked,
			DNSSEC:      dnssecLabel(r),
			Score:       r.Score,
		})
	}
	return json.MarshalIndent(rep, "", "  ")
}

// CSV writes a spreadsheet-friendly report.
func CSV(w io.Writer, rows []rank.Row) error {
	cw := csv.NewWriter(w)
	header := []string{
		"rank", "name", "address", "system",
		"cached_min_ms", "cached_avg_ms", "cached_max_ms", "cached_p50_ms", "cached_p95_ms", "cached_stddev_ms",
		"uncached_min_ms", "uncached_avg_ms", "uncached_max_ms", "uncached_p50_ms", "uncached_p95_ms", "uncached_stddev_ms",
		"tld_min_ms", "tld_avg_ms", "tld_max_ms", "tld_p50_ms", "tld_p95_ms", "tld_stddev_ms",
		"reliability", "successes", "attempts", "nx_rewrite", "dnssec", "score",
	}
	if err := cw.Write(header); err != nil {
		return err
	}
	for i, r := range rows {
		rec := []string{
			fmt.Sprintf("%d", i+1),
			r.Name,
			r.Address,
			fmt.Sprintf("%t", r.System),
			fmtMS(r.Cached.Min), fmtMS(r.Cached.Avg), fmtMS(r.Cached.Max), fmtMS(r.Cached.P50), fmtMS(r.Cached.P95), fmtMS(r.Cached.StdDev),
			fmtMS(r.Uncached.Min), fmtMS(r.Uncached.Avg), fmtMS(r.Uncached.Max), fmtMS(r.Uncached.P50), fmtMS(r.Uncached.P95), fmtMS(r.Uncached.StdDev),
			fmtMS(r.TLD.Min), fmtMS(r.TLD.Avg), fmtMS(r.TLD.Max), fmtMS(r.TLD.P50), fmtMS(r.TLD.P95), fmtMS(r.TLD.StdDev),
			fmt.Sprintf("%.4f", r.Reliability),
			fmt.Sprintf("%d", r.Successes),
			fmt.Sprintf("%d", r.Attempts),
			fmt.Sprintf("%t", r.NXRewrite),
			dnssecLabel(r),
			fmt.Sprintf("%.6f", r.Score),
		}
		if err := cw.Write(rec); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// HTML writes a self-contained lab-style report (original chrome, not a product clone).
func HTML(rows []rank.Row, mode rank.Mode, now time.Time) []byte {
	var b bytes.Buffer
	b.WriteString("<!DOCTYPE html>\n<html lang=\"en\"><head><meta charset=\"utf-8\">")
	b.WriteString("<title>dns-bench report</title><style>")
	b.WriteString(`
:root { --ink:#d7f7ee; --muted:#8eaea6; --bg:#101714; --card:#18211e; --accent:#3ee0b8; --warn:#e0a33e; }
html,body { background:var(--bg); color:var(--ink); font:14px/1.5 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; margin:0; }
main { max-width:1100px; margin:2rem auto; padding:0 1.25rem 3rem; }
h1 { font-weight:600; letter-spacing:.04em; }
.tag { color:var(--accent); }
.disc { color:var(--muted); font-size:12px; max-width:70ch; }
table { width:100%; border-collapse:collapse; background:var(--card); margin-top:1.5rem; }
th,td { text-align:left; padding:.55rem .7rem; border-bottom:1px solid #24332e; }
th { color:var(--muted); font-weight:500; font-size:12px; }
.bar { height:8px; background:#0c1210; border-radius:99px; overflow:hidden; min-width:80px; }
.bar > span { display:block; height:100%; background:var(--accent); }
.warn { color:var(--warn); }
`)
	b.WriteString("</style></head><body><main>")
	fmt.Fprintf(&b, "<p class=\"tag\">dns-bench v%s · resolver lab</p>", html.EscapeString(version.Version))
	b.WriteString("<h1>Local DNS resolver results</h1>")
	fmt.Fprintf(&b, "<p>Generated %s · rank mode %s</p>", html.EscapeString(now.UTC().Format(time.RFC3339)), html.EscapeString(mode.String()))
	fmt.Fprintf(&b, "<p>%s</p>", html.EscapeString(SummaryLine(rows, mode)))
	fmt.Fprintf(&b, "<p class=\"disc\">%s</p>", html.EscapeString(Disclaimer))
	b.WriteString("<table><thead><tr>")
	for _, h := range []string{"Rank", "Resolver", "Cached p50", "Uncached p50", "TLD p50", "Loss", "NX", "DNSSEC", "Score", "Relative"} {
		fmt.Fprintf(&b, "<th>%s</th>", h)
	}
	b.WriteString("</tr></thead><tbody>")
	maxBlend := 0.0
	for _, r := range rows {
		if r.Score > maxBlend {
			maxBlend = r.Score
		}
	}
	for i, r := range rows {
		nx := "—"
		nxClass := ""
		if r.NXChecked || r.NXRewrite {
			if r.NXRewrite {
				nx = "rewrite"
				nxClass = " class=\"warn\""
			} else {
				nx = "ok"
			}
		}
		dnssec := "—"
		if r.DNSSECChecked {
			if r.DNSSECValidate {
				dnssec = "validating"
			} else {
				dnssec = "no"
			}
		}
		pct := 0.0
		if maxBlend > 0 {
			pct = 100 * r.Score / maxBlend
		}
		fmt.Fprintf(&b, "<tr><td>%d</td><td>%s %s</td><td>%s ms</td><td>%s ms</td><td>%s ms</td><td>%.1f%%</td><td%s>%s</td><td>%s</td><td>%.3f</td><td><div class=\"bar\"><span style=\"width:%.1f%%\"></span></div></td></tr>",
			i+1,
			html.EscapeString(r.Name),
			html.EscapeString(r.Address),
			html.EscapeString(fmtMS(r.Cached.P50)),
			html.EscapeString(fmtMS(r.Uncached.P50)),
			html.EscapeString(fmtMS(r.TLD.P50)),
			(1-r.Reliability)*100,
			nxClass,
			html.EscapeString(nx),
			html.EscapeString(dnssec),
			r.Score,
			pct,
		)
	}
	b.WriteString("</tbody></table></main></body></html>\n")
	return b.Bytes()
}
