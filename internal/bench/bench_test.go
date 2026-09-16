package bench_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/78tacos/dns-bench/internal/bench"
	"github.com/78tacos/dns-bench/internal/dnsquery"
	"github.com/78tacos/dns-bench/internal/resolvers"
)

type fakeDNS struct {
	mu          sync.Mutex
	names       []string
	uncachedRTT time.Duration
	cachedRTT   time.Duration
	failAddr    string
	rewrite     bool
	dropAll     bool
}

func (f *fakeDNS) Query(_ context.Context, server, qname string, _ time.Duration) dnsquery.Result {
	f.mu.Lock()
	f.names = append(f.names, server+" "+qname)
	f.mu.Unlock()

	if f.dropAll || server == f.failAddr {
		return dnsquery.Result{Err: errors.New("timeout"), RTT: 5 * time.Millisecond}
	}
	if strings.HasSuffix(qname, ".invalid") {
		if f.rewrite {
			return dnsquery.Result{
				RTT:     f.cachedRTT,
				RCode:   dnsquery.RCodeNoError,
				Answers: []dnsquery.ARecord{{IP: "203.0.113.1"}},
			}
		}
		return dnsquery.Result{RTT: f.cachedRTT, RCode: dnsquery.RCodeNXDOMAIN}
	}
	if strings.HasPrefix(qname, "u-") {
		return dnsquery.Result{RTT: f.uncachedRTT, RCode: dnsquery.RCodeNXDOMAIN}
	}
	return dnsquery.Result{
		RTT:     f.cachedRTT,
		RCode:   dnsquery.RCodeNoError,
		Answers: []dnsquery.ARecord{{IP: "192.0.2.1"}},
	}
}

func TestRunCachedUncachedAndReliability(t *testing.T) {
	f := &fakeDNS{
		uncachedRTT: 40 * time.Millisecond,
		cachedRTT:   10 * time.Millisecond,
		failAddr:    "192.0.2.9:53",
	}
	cfg := bench.Config{
		Resolvers: []resolvers.Resolver{
			{Name: "Good", IP: "192.0.2.1", Port: "53"},
			{Name: "Dead", IP: "192.0.2.9", Port: "53"},
		},
		Domains: []string{"example.com", "iana.org"},
		Queries: 4,
		Timeout: time.Second,
		CheckNX: true,
		RunID:   "abc123",
		Query:   f.Query,
	}
	ms := bench.Run(context.Background(), cfg)
	if len(ms) != 2 {
		t.Fatalf("len %d", len(ms))
	}

	var good, dead bench.Measurement
	for _, m := range ms {
		switch m.Resolver.Name {
		case "Good":
			good = m
		case "Dead":
			dead = m
		}
	}
	if good.Attempts != 8 {
		t.Fatalf("good attempts %d (4 uncached + 4 cached timed; warmup not counted)", good.Attempts)
	}
	if good.Successes != 8 || good.Cached.Failures != 0 {
		t.Fatalf("good successes %+v", good)
	}
	if good.Cached.P50 != 10*time.Millisecond {
		t.Fatalf("cached p50 %v", good.Cached.P50)
	}
	if good.Uncached.P50 != 40*time.Millisecond {
		t.Fatalf("uncached p50 %v", good.Uncached.P50)
	}
	if !good.NXChecked || good.NXRewrite {
		t.Fatalf("nx flags %+v", good)
	}
	if dead.Successes != 0 || dead.Attempts != 8 {
		t.Fatalf("dead %+v", dead)
	}

	f.mu.Lock()
	names := append([]string(nil), f.names...)
	f.mu.Unlock()
	var warmup int
	var unique int
	for _, n := range names {
		if strings.Contains(n, "u-abc123-") {
			unique++
		}
		if strings.HasSuffix(n, " example.com") || strings.HasSuffix(n, " iana.org") {
			warmup++
		}
	}
	if unique < 4 {
		t.Fatalf("expected unique uncached qnames, got %v", names)
	}
	want := bench.UncachedName("abc123", 0, "example.com")
	if want != "u-abc123-0.example.com" {
		t.Fatalf("shape %s", want)
	}
}

func TestNXRewriteDetection(t *testing.T) {
	f := &fakeDNS{
		uncachedRTT: time.Millisecond,
		cachedRTT:   time.Millisecond,
		rewrite:     true,
	}
	cfg := bench.Config{
		Resolvers: []resolvers.Resolver{{Name: "ISP", IP: "192.0.2.2", Port: "53"}},
		Domains:   []string{"example.com"},
		Queries:   1,
		CheckNX:   true,
		RunID:     "zz",
		Query:     f.Query,
	}
	ms := bench.Run(context.Background(), cfg)
	if !ms[0].NXRewrite {
		t.Fatalf("expected rewrite: %+v", ms[0])
	}
}

func TestUncachedName(t *testing.T) {
	got := bench.UncachedName("deadbeef", 3, "google.com")
	if got != "u-deadbeef-3.google.com" {
		t.Fatalf("got %s", got)
	}
}
