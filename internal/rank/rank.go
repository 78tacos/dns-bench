package rank

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/78tacos/dns-bench/internal/stats"
)

// Mode selects how resolvers are scored. Higher score is better.
type Mode int

const (
	ModeBlended Mode = iota
	ModeCached
	ModeUncached
	ModeReliability
)

// NXRewritePenalty multiplies score when a resolver rewrites NXDOMAIN
// into an A record (search/assist interception). Documented in README.
const NXRewritePenalty = 0.85

// Input is one resolver's measured stats, before scoring.
type Input struct {
	Name      string
	Address   string
	System    bool
	Cached    stats.Summary
	Uncached  stats.Summary
	Successes int
	Attempts  int
	NXRewrite bool
	NXChecked bool
}

// Row is a scored, ready-to-print resolver result.
type Row struct {
	Input
	Reliability float64
	Score       float64
}

// ParseMode accepts blended|cached|uncached|reliability (aliases allowed).
func ParseMode(s string) (Mode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "blended", "blend":
		return ModeBlended, nil
	case "cached", "cache":
		return ModeCached, nil
	case "uncached", "cold":
		return ModeUncached, nil
	case "reliability", "reliable", "loss":
		return ModeReliability, nil
	default:
		return 0, fmt.Errorf("unknown rank mode %q (want blended, cached, uncached, reliability)", s)
	}
}

func (m Mode) String() string {
	switch m {
	case ModeBlended:
		return "blended"
	case ModeCached:
		return "cached"
	case ModeUncached:
		return "uncached"
	case ModeReliability:
		return "reliability"
	default:
		panic(fmt.Sprintf("unhandled rank.Mode: %d", m))
	}
}

// Reliability is successes/attempts, or 0 if attempts is 0.
func Reliability(successes, attempts int) float64 {
	if attempts <= 0 {
		return 0
	}
	return float64(successes) / float64(attempts)
}

func latencyMS(d time.Duration) float64 {
	if d <= 0 {
		return math.Inf(1)
	}
	return float64(d) / float64(time.Millisecond)
}

func blendedMS(cached, uncached time.Duration) float64 {
	c, u := latencyMS(cached), latencyMS(uncached)
	switch {
	case math.IsInf(c, 1) && math.IsInf(u, 1):
		return math.Inf(1)
	case math.IsInf(c, 1):
		return u
	case math.IsInf(u, 1):
		return c
	default:
		return 0.5*c + 0.5*u
	}
}

// Score is higher-is-better. Latency modes use reliability / milliseconds
// so lossy resolvers drop even if their few successes were fast.
func Score(rel float64, cachedP50, uncachedP50 time.Duration, nxRewrite bool, mode Mode) float64 {
	if rel <= 0 {
		return 0
	}
	var base float64
	switch mode {
	case ModeBlended:
		base = rel / blendedMS(cachedP50, uncachedP50)
	case ModeCached:
		base = rel / latencyMS(cachedP50)
	case ModeUncached:
		base = rel / latencyMS(uncachedP50)
	case ModeReliability:
		base = rel
	default:
		panic(fmt.Sprintf("unhandled rank.Mode: %d", mode))
	}
	if math.IsInf(base, 0) || math.IsNaN(base) {
		return 0
	}
	if nxRewrite {
		base *= NXRewritePenalty
	}
	return base
}

// Apply scores and sorts a copy of inputs (highest score first).
func Apply(inputs []Input, mode Mode) []Row {
	out := make([]Row, len(inputs))
	for i, in := range inputs {
		rel := Reliability(in.Successes, in.Attempts)
		out[i] = Row{
			Input:       in,
			Reliability: rel,
			Score:       Score(rel, in.Cached.P50, in.Uncached.P50, in.NXRewrite, mode),
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		bi := blendedMS(out[i].Cached.P50, out[i].Uncached.P50)
		bj := blendedMS(out[j].Cached.P50, out[j].Uncached.P50)
		if bi != bj {
			return bi < bj
		}
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Address < out[j].Address
	})
	return out
}
