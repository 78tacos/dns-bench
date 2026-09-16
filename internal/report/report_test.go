package report_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/78tacos/dns-bench/internal/rank"
	"github.com/78tacos/dns-bench/internal/report"
	"github.com/78tacos/dns-bench/internal/stats"
)

func sampleRows() []rank.Row {
	in := []rank.Input{
		{
			Name: "Fast", Address: "192.0.2.1:53", System: true,
			Cached: stats.Summary{Count: 4, P50: 10 * time.Millisecond, P95: 12 * time.Millisecond,
				Min: 8 * time.Millisecond, Avg: 10 * time.Millisecond, Max: 12 * time.Millisecond},
			Uncached:  stats.Summary{Count: 4, P50: 20 * time.Millisecond, P95: 30 * time.Millisecond},
			TLD:       stats.Summary{Count: 4, P50: 40 * time.Millisecond, P95: 55 * time.Millisecond},
			Successes: 12, Attempts: 12,
		},
		{
			Name: "Slow", Address: "192.0.2.2:53",
			Cached:    stats.Summary{Count: 3, P50: 80 * time.Millisecond, P95: 90 * time.Millisecond},
			Uncached:  stats.Summary{Count: 3, P50: 100 * time.Millisecond, P95: 120 * time.Millisecond},
			Successes: 6, Attempts: 8, NXChecked: true, NXRewrite: true,
		},
	}
	return rank.Apply(in, rank.ModeBlended)
}

func TestTableAndSummary(t *testing.T) {
	rows := sampleRows()
	tbl := report.Table(rows)
	if !strings.Contains(tbl, "RANK") || !strings.Contains(tbl, "Fast") || !strings.Contains(tbl, "TLD") {
		t.Fatalf("table:\n%s", tbl)
	}
	sum := report.SummaryLine(rows, rank.ModeBlended)
	if !strings.Contains(sum, "Fast") || strings.Contains(strings.ToLower(sum), "conclusions") {
		t.Fatalf("summary: %s", sum)
	}
	if report.SummaryLine(nil, rank.ModeBlended) == "" {
		t.Fatal("empty summary")
	}
}

func TestTableUncheckedNXAndDNSSECAreDashes(t *testing.T) {
	rows := rank.Apply([]rank.Input{
		{Name: "Silent", Address: "192.0.2.8:53", Successes: 0, Attempts: 4},
	}, rank.ModeBlended)
	tbl := report.Table(rows)
	if strings.Contains(tbl, " ok") || strings.Contains(tbl, "rewrite") {
		t.Fatalf("unchecked NX should not look clean:\n%s", tbl)
	}
	if strings.Contains(tbl, " yes") || strings.Contains(tbl, " no") {
		t.Fatalf("unchecked DNSSEC should not look like a result:\n%s", tbl)
	}
	if !strings.Contains(tbl, "—") {
		t.Fatalf("expected em dash for unchecked fields:\n%s", tbl)
	}
}

func TestJSONCSVHTML(t *testing.T) {
	rows := sampleRows()
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	raw, err := report.JSON(rows, rank.ModeBlended, now)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["tool"] != "dns-bench" {
		t.Fatalf("tool %v", doc["tool"])
	}
	if !strings.Contains(doc["disclaimer"].(string), "not affiliated") {
		t.Fatal("disclaimer")
	}
	if strings.Contains(strings.ToLower(string(raw)), "gibson research") == false {
		// disclaimer mentions GRC by name to be explicit
		t.Fatal("expected GRC named in disclaimer")
	}

	var buf bytes.Buffer
	if err := report.CSV(&buf, rows); err != nil {
		t.Fatal(err)
	}
	csv := buf.String()
	if !strings.HasPrefix(csv, "rank,name,address") {
		t.Fatalf("csv header: %s", csv)
	}
	if !strings.Contains(csv, "tld_p50_ms") || !strings.Contains(csv, "Fast") {
		t.Fatal("csv tld column or body")
	}

	htmlDoc := string(report.HTML(rows, rank.ModeBlended, now))
	if !strings.Contains(htmlDoc, "dns-bench") || !strings.Contains(htmlDoc, "not affiliated") {
		t.Fatal("html branding/disclaimer")
	}
	if strings.Contains(htmlDoc, "GRC-compatible") {
		t.Fatal("must not claim GRC-compatible")
	}
}

func TestDisclaimerConstant(t *testing.T) {
	d := strings.ToLower(report.Disclaimer)
	if strings.Contains(d, "affiliated") == false {
		t.Fatal(report.Disclaimer)
	}
	if strings.Contains(d, "dns benchmark") == false {
		t.Fatal("should mention GRC product name to disclaim it")
	}
}
