package stats

import (
	"math"
	"sort"
	"time"
)

// Summary is latency statistics over successful samples only.
type Summary struct {
	Count  int
	Min    time.Duration
	Avg    time.Duration
	Max    time.Duration
	P50    time.Duration
	P95    time.Duration
	StdDev time.Duration
}

// Summarize computes min/avg/max and nearest-rank p50/p95.
// samples may be unsorted.
func Summarize(samples []time.Duration) Summary {
	s := Summary{Count: len(samples)}
	if len(samples) == 0 {
		return s
	}
	sorted := append([]time.Duration(nil), samples...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	s.Min = sorted[0]
	s.Max = sorted[len(sorted)-1]
	var sum time.Duration
	for _, d := range sorted {
		sum += d
	}
	s.Avg = sum / time.Duration(len(sorted))
	s.P50 = Percentile(sorted, 50)
	s.P95 = Percentile(sorted, 95)
	s.StdDev = stdDev(sorted, s.Avg)
	return s
}

func stdDev(sorted []time.Duration, mean time.Duration) time.Duration {
	n := len(sorted)
	if n < 2 {
		return 0
	}
	var ss float64
	m := float64(mean)
	for _, d := range sorted {
		diff := float64(d) - m
		ss += diff * diff
	}
	return time.Duration(math.Sqrt(ss / float64(n-1)))
}

// Percentile returns the linear-interpolated percentile of a sorted
// non-empty slice. p is in 0..100. Callers must sort samples first.
func Percentile(sorted []time.Duration, p float64) time.Duration {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[n-1]
	}
	idx := (p / 100) * float64(n-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	w := idx - float64(lo)
	return time.Duration(float64(sorted[lo])*(1-w) + float64(sorted[hi])*w)
}

// Milliseconds renders d as fractional milliseconds.
func Milliseconds(d time.Duration) float64 {
	if d <= 0 {
		return 0
	}
	return float64(d) / float64(time.Millisecond)
}
