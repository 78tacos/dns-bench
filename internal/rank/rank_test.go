package rank_test

import (
	"math"
	"testing"
	"time"

	"github.com/78tacos/dns-bench/internal/rank"
	"github.com/78tacos/dns-bench/internal/stats"
)

func TestParseMode(t *testing.T) {
	cases := []struct {
		in      string
		want    rank.Mode
		wantErr bool
	}{
		{"", rank.ModeBlended, false},
		{"blended", rank.ModeBlended, false},
		{"BLEND", rank.ModeBlended, false},
		{"cached", rank.ModeCached, false},
		{"uncached", rank.ModeUncached, false},
		{"cold", rank.ModeUncached, false},
		{"tld", rank.ModeTLD, false},
		{"dotcom", rank.ModeTLD, false},
		{"reliability", rank.ModeReliability, false},
		{"loss", rank.ModeReliability, false},
		{"nope", 0, true},
	}
	for _, tc := range cases {
		got, err := rank.ParseMode(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("%q: expected error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("%q: got %v want %v", tc.in, got, tc.want)
		}
		if got.String() == "" {
			t.Fatalf("%q: empty String()", tc.in)
		}
	}
}

func TestModeStringExhaustive(t *testing.T) {
	for _, m := range []rank.Mode{
		rank.ModeBlended,
		rank.ModeCached,
		rank.ModeUncached,
		rank.ModeTLD,
		rank.ModeReliability,
	} {
		if m.String() == "" {
			t.Fatalf("mode %d empty", m)
		}
	}
}

func TestReliability(t *testing.T) {
	if rank.Reliability(0, 0) != 0 {
		t.Fatal("zero attempts")
	}
	if rank.Reliability(8, 10) != 0.8 {
		t.Fatal("8/10")
	}
}

func scored(rel float64, cached, uncached time.Duration, nx bool, mode rank.Mode) float64 {
	attempts := 10
	successes := int(rel*float64(attempts) + 0.5)
	in := rank.Input{
		Cached:    stats.Summary{Count: 1, P50: cached},
		Uncached:  stats.Summary{Count: 1, P50: uncached},
		Successes: successes,
		Attempts:  attempts,
		NXRewrite: nx,
	}
	return rank.Score(in, mode)
}

func TestScoreLatencyModes(t *testing.T) {
	fast := scored(1, 10*time.Millisecond, 20*time.Millisecond, false, rank.ModeBlended)
	slow := scored(1, 50*time.Millisecond, 100*time.Millisecond, false, rank.ModeBlended)
	if fast <= slow {
		t.Fatalf("fast %v should beat slow %v", fast, slow)
	}
	flaky := scored(0.5, 10*time.Millisecond, 20*time.Millisecond, false, rank.ModeBlended)
	if flaky >= fast {
		t.Fatalf("flaky %v should lose to reliable %v", flaky, fast)
	}
	if scored(0, 1*time.Millisecond, 1*time.Millisecond, false, rank.ModeBlended) != 0 {
		t.Fatal("zero reliability")
	}
}

func TestScoreCachedIgnoresUncached(t *testing.T) {
	a := scored(1, 10*time.Millisecond, 500*time.Millisecond, false, rank.ModeCached)
	b := scored(1, 10*time.Millisecond, 1*time.Millisecond, false, rank.ModeCached)
	if a != b {
		t.Fatalf("cached mode should ignore uncached: %v vs %v", a, b)
	}
}

func TestScoreUncachedIgnoresCached(t *testing.T) {
	a := scored(1, 500*time.Millisecond, 20*time.Millisecond, false, rank.ModeUncached)
	b := scored(1, 1*time.Millisecond, 20*time.Millisecond, false, rank.ModeUncached)
	if a != b {
		t.Fatalf("uncached mode should ignore cached: %v vs %v", a, b)
	}
}

func TestScoreTLDMode(t *testing.T) {
	fast := rank.Score(rank.Input{
		TLD:       stats.Summary{Count: 3, P50: 30 * time.Millisecond},
		Successes: 10, Attempts: 10,
	}, rank.ModeTLD)
	slow := rank.Score(rank.Input{
		TLD:       stats.Summary{Count: 3, P50: 90 * time.Millisecond},
		Cached:    stats.Summary{Count: 3, P50: 1 * time.Millisecond},
		Successes: 10, Attempts: 10,
	}, rank.ModeTLD)
	if fast <= slow {
		t.Fatalf("tld mode should rank by TLD p50: %v vs %v", fast, slow)
	}
}

func TestScoreReliabilityMode(t *testing.T) {
	if scored(0.9, 5*time.Millisecond, 5*time.Millisecond, false, rank.ModeReliability) != 0.9 {
		t.Fatal("reliability mode is raw ratio")
	}
}

func TestNXRewritePenalty(t *testing.T) {
	clean := scored(1, 10*time.Millisecond, 10*time.Millisecond, false, rank.ModeBlended)
	dirty := scored(1, 10*time.Millisecond, 10*time.Millisecond, true, rank.ModeBlended)
	want := clean * rank.NXRewritePenalty
	if math.Abs(dirty-want) > 1e-9 {
		t.Fatalf("penalty: dirty %v want %v", dirty, want)
	}
}

func ms(n int) time.Duration { return time.Duration(n) * time.Millisecond }

func sample(p50 time.Duration) stats.Summary {
	return stats.Summary{Count: 4, P50: p50, P95: p50 + ms(5), Min: p50, Max: p50, Avg: p50}
}

func TestApplyOrder(t *testing.T) {
	inputs := []rank.Input{
		{Name: "slow-reliable", Address: "10.0.0.1", Cached: sample(ms(50)), Uncached: sample(ms(100)), Successes: 10, Attempts: 10},
		{Name: "fast-flaky", Address: "10.0.0.2", Cached: sample(ms(5)), Uncached: sample(ms(10)), Successes: 3, Attempts: 10},
		{Name: "fast-reliable", Address: "10.0.0.3", Cached: sample(ms(10)), Uncached: sample(ms(20)), Successes: 10, Attempts: 10},
		{Name: "dead", Address: "10.0.0.4", Successes: 0, Attempts: 10},
	}
	rows := rank.Apply(inputs, rank.ModeBlended)
	if len(rows) != 4 {
		t.Fatalf("len %d", len(rows))
	}
	if rows[0].Name != "fast-reliable" {
		t.Fatalf("want fast-reliable first, got %s", rows[0].Name)
	}
	if rows[1].Name != "fast-flaky" {
		t.Fatalf("want fast-flaky second (still faster enough to beat slow-reliable), got %s", rows[1].Name)
	}
	if rows[2].Name != "slow-reliable" {
		t.Fatalf("want slow-reliable third, got %s", rows[2].Name)
	}
	if rows[3].Name != "dead" {
		t.Fatalf("dead should be last, got %s", rows[3].Name)
	}
	if rows[3].Score != 0 {
		t.Fatalf("dead score %v", rows[3].Score)
	}
}

func TestApplyReliabilityModePrefersStable(t *testing.T) {
	inputs := []rank.Input{
		{Name: "slow-reliable", Address: "10.0.0.1", Cached: sample(ms(50)), Uncached: sample(ms(100)), Successes: 10, Attempts: 10},
		{Name: "fast-flaky", Address: "10.0.0.2", Cached: sample(ms(5)), Uncached: sample(ms(10)), Successes: 3, Attempts: 10},
	}
	rows := rank.Apply(inputs, rank.ModeReliability)
	if rows[0].Name != "slow-reliable" {
		t.Fatalf("reliability mode should prefer 100%% replies, got %s", rows[0].Name)
	}
}

func TestApplyTieBreakByName(t *testing.T) {
	inputs := []rank.Input{
		{Name: "Zed", Address: "10.0.0.2", Cached: sample(ms(10)), Uncached: sample(ms(10)), Successes: 1, Attempts: 1},
		{Name: "Amy", Address: "10.0.0.1", Cached: sample(ms(10)), Uncached: sample(ms(10)), Successes: 1, Attempts: 1},
	}
	rows := rank.Apply(inputs, rank.ModeBlended)
	if rows[0].Name != "Amy" {
		t.Fatalf("tie-break name: got %s", rows[0].Name)
	}
}
