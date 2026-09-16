package bench

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/78tacos/dns-bench/internal/dnsquery"
	"github.com/78tacos/dns-bench/internal/resolvers"
	"github.com/78tacos/dns-bench/internal/stats"
)

// QueryFunc is injectable so tests can mock timings without the internet.
type QueryFunc func(ctx context.Context, server, qname string, timeout time.Duration) dnsquery.Result

// DNSSECProbeName is a public name whose zone is deliberately DNSSEC-broken.
// Validating resolvers typically return SERVFAIL; non-validating resolvers
// often still return an A record.
const DNSSECProbeName = "dnssec-failed.org"

// Config controls a run.
type Config struct {
	Resolvers   []resolvers.Resolver
	Domains     []string
	Queries     int
	Timeout     time.Duration
	CheckNX     bool
	CheckTLD    bool
	CheckDNSSEC bool
	RunID       string
	Query       QueryFunc
	OnDone      func(Measurement)
}

// Measurement is one resolver's raw results (unranked).
type Measurement struct {
	Resolver       resolvers.Resolver
	Cached         stats.Summary
	Uncached       stats.Summary
	TLD            stats.Summary
	Successes      int
	Attempts       int
	NXRewrite      bool
	NXChecked      bool
	NXRCode        dnsquery.RCode
	DNSSECChecked  bool
	DNSSECValidate bool
	DNSSECRCode    dnsquery.RCode
}

// Run benchmarks each resolver. Resolvers are probed in parallel; queries
// to a single resolver stay serial so cache warmup is ordered.
func Run(ctx context.Context, cfg Config) []Measurement {
	if cfg.Query == nil {
		cfg.Query = dnsquery.QueryA
	}
	if cfg.Queries < 1 {
		cfg.Queries = 1
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 2 * time.Second
	}
	if cfg.RunID == "" {
		cfg.RunID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	domains := cfg.Domains
	if len(domains) == 0 {
		domains = []string{"example.com"}
	}

	out := make([]Measurement, len(cfg.Resolvers))
	var wg sync.WaitGroup
	for i, r := range cfg.Resolvers {
		wg.Add(1)
		go func(i int, r resolvers.Resolver) {
			defer wg.Done()
			m := runOne(ctx, cfg, r, domains)
			out[i] = m
			if cfg.OnDone != nil {
				cfg.OnDone(m)
			}
		}(i, r)
	}
	wg.Wait()
	return out
}

func runOne(ctx context.Context, cfg Config, r resolvers.Resolver, domains []string) Measurement {
	m := Measurement{Resolver: r}
	addr := r.Addr()

	var uncached []time.Duration
	var uncachedFail int
	for i := 0; i < cfg.Queries; i++ {
		m.Attempts++
		qname := uncachedName(cfg.RunID, i, domains[i%len(domains)])
		res := cfg.Query(ctx, addr, qname, cfg.Timeout)
		if replyOK(res) {
			m.Successes++
			uncached = append(uncached, res.RTT)
		} else {
			uncachedFail++
		}
	}
	m.Uncached = stats.Summarize(uncached, uncachedFail)

	var cached []time.Duration
	var cachedFail int
	for i := 0; i < cfg.Queries; i++ {
		qname := domains[i%len(domains)]
		_ = cfg.Query(ctx, addr, qname, cfg.Timeout) // warmup; not scored
		m.Attempts++
		res := cfg.Query(ctx, addr, qname, cfg.Timeout)
		if cachedOK(res) {
			m.Successes++
			cached = append(cached, res.RTT)
		} else {
			cachedFail++
		}
	}
	m.Cached = stats.Summarize(cached, cachedFail)

	if cfg.CheckTLD {
		var tld []time.Duration
		var tldFail int
		for i := 0; i < cfg.Queries; i++ {
			m.Attempts++
			qname := tldName(cfg.RunID, i)
			res := cfg.Query(ctx, addr, qname, cfg.Timeout)
			if replyOK(res) {
				m.Successes++
				tld = append(tld, res.RTT)
				if len(res.Answers) > 0 {
					m.NXRewrite = true
				}
			} else {
				tldFail++
			}
		}
		m.TLD = stats.Summarize(tld, tldFail)
	}

	if cfg.CheckNX {
		qname := fmt.Sprintf("nx-%s.invalid", cfg.RunID)
		res := cfg.Query(ctx, addr, qname, cfg.Timeout)
		if replyOK(res) {
			m.NXChecked = true
			m.NXRCode = res.RCode
			if len(res.Answers) > 0 {
				m.NXRewrite = true
			}
		}
	}

	if cfg.CheckDNSSEC {
		res := cfg.Query(ctx, addr, DNSSECProbeName, cfg.Timeout)
		if replyOK(res) {
			m.DNSSECChecked = true
			m.DNSSECRCode = res.RCode
			if res.RCode == dnsquery.RCodeServFail {
				m.DNSSECValidate = true
			}
		}
	}
	return m
}

func uncachedName(runID string, i int, domain string) string {
	runID = strings.Trim(runID, ".")
	return fmt.Sprintf("u-%s-%d.%s", runID, i, domain)
}

func tldName(runID string, i int) string {
	runID = strings.Trim(runID, ".")
	return fmt.Sprintf("c-%s-%d.com", runID, i)
}

func replyOK(res dnsquery.Result) bool {
	return res.Err == nil
}

func cachedOK(res dnsquery.Result) bool {
	return res.Err == nil && res.RCode == dnsquery.RCodeNoError && len(res.Answers) > 0
}

// UncachedName is exported for tests that assert QNAME shape.
func UncachedName(runID string, i int, domain string) string {
	return uncachedName(runID, i, domain)
}

// TLDName is a unique .com SLD used for TLD-path timing.
func TLDName(runID string, i int) string {
	return tldName(runID, i)
}
