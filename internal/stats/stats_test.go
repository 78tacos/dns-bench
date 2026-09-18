package stats_test

import (
	"testing"
	"time"

	"github.com/78tacos/dns-bench/internal/stats"
)

func TestPercentileEmptyAndSingle(t *testing.T) {
	if got := stats.Percentile(nil, 50); got != 0 {
		t.Fatalf("empty: got %v", got)
	}
	one := []time.Duration{12 * time.Millisecond}
	if got := stats.Percentile(one, 95); got != 12*time.Millisecond {
		t.Fatalf("single: got %v", got)
	}
}

func TestPercentileInterpolated(t *testing.T) {
	// Four samples: 10, 20, 30, 40 ms. p50 is midpoint of 20 and 30.
	sorted := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
	}
	p50 := stats.Percentile(sorted, 50)
	if p50 != 25*time.Millisecond {
		t.Fatalf("p50: got %v want 25ms", p50)
	}
	p0 := stats.Percentile(sorted, 0)
	if p0 != 10*time.Millisecond {
		t.Fatalf("p0: got %v", p0)
	}
	p100 := stats.Percentile(sorted, 100)
	if p100 != 40*time.Millisecond {
		t.Fatalf("p100: got %v", p100)
	}
}

func TestSummarize(t *testing.T) {
	samples := []time.Duration{
		40 * time.Millisecond,
		10 * time.Millisecond,
		30 * time.Millisecond,
		20 * time.Millisecond,
	}
	s := stats.Summarize(samples)
	if s.Count != 4 {
		t.Fatalf("counts: %+v", s)
	}
	if s.Min != 10*time.Millisecond || s.Max != 40*time.Millisecond {
		t.Fatalf("min/max: %+v", s)
	}
	if s.Avg != 25*time.Millisecond {
		t.Fatalf("avg: %v", s.Avg)
	}
	if s.P50 != 25*time.Millisecond {
		t.Fatalf("p50: %v", s.P50)
	}
	// Sample stddev of 10,20,30,40 ms is ~12.91 ms.
	sdMS := float64(s.StdDev) / float64(time.Millisecond)
	if sdMS < 12.5 || sdMS > 13.3 {
		t.Fatalf("stddev ms: %v (%v)", sdMS, s.StdDev)
	}
}

func TestSummarizeEmpty(t *testing.T) {
	s := stats.Summarize(nil)
	if s.Count != 0 || s.P50 != 0 {
		t.Fatalf("empty: %+v", s)
	}
}

func TestMilliseconds(t *testing.T) {
	if stats.Milliseconds(0) != 0 {
		t.Fatal("zero")
	}
	if stats.Milliseconds(-time.Second) != 0 {
		t.Fatal("negative")
	}
	ms := stats.Milliseconds(1500 * time.Microsecond)
	if ms < 1.4 || ms > 1.6 {
		t.Fatalf("ms: %v", ms)
	}
}
