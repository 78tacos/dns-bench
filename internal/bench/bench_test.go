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
	mu             sync.Mutex
	names          []string
	uncachedRTT    time.Duration
	cachedRTT      time.Duration
	tldRTT         time.Duration
	failAddr       string
	rewrite        bool
	dropAll        bool
	dnssecSERVFAIL bool
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
	if strings.EqualFold(qname, bench.DNSSECProbeName) {
		if f.dnssecSERVFAIL {
			return dnsquery.Result{RTT: f.cachedRTT, RCode: dnsquery.RCodeServFail}
		}
		return dnsquery.Result{
			RTT:     f.cachedRTT,
			RCode:   dnsquery.RCodeNoError,
			Answers: []dnsquery.ARecord{{IP: "192.0.2.99"}},
		}
	}
	if strings.HasPrefix(qname, "c-") && strings.HasSuffix(qname, ".com") {
		rtt := f.tldRTT
		if rtt == 0 {
			rtt = 80 * time.Millisecond
		}
		if f.rewrite {
			return dnsquery.Result{
				RTT:     rtt,
				RCode:   dnsquery.RCodeNoError,
				Answers: []dnsquery.ARecord{{IP: "203.0.113.1"}},
			}
		}
		return dnsquery.Result{RTT: rtt, RCode: dnsquery.RCodeNXDOMAIN}
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

func TestRunCachedUncachedTLDAndReliability(t *testing.T) {
	f := &fakeDNS{
		uncachedRTT:    40 * time.Millisecond,
		cachedRTT:      10 * time.Millisecond,
		tldRTT:         80 * time.Millisecond,
		failAddr:       "192.0.2.9:53",
		dnssecSERVFAIL: true,
	}
	cfg := bench.Config{
		Resolvers: []resolvers.Resolver{
			{Name: "Good", IP: "192.0.2.1", Port: "53"},
			{Name: "Dead", IP: "192.0.2.9", Port: "53"},
		},
		Domains:     []string{"example.com", "iana.org"},
		Queries:     4,
		Timeout:     time.Second,
		CheckNX:     true,
		CheckTLD:    true,
		CheckDNSSEC: true,
		RunID:       "abc123",
		Query:       f.Query,
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
	if good.Attempts != 12 {
		t.Fatalf("good attempts %d (4 uncached + 4 cached timed + 4 tld; warmup not counted)", good.Attempts)
	}
	if good.Successes != 12 || good.Cached.Failures != 0 {
		t.Fatalf("good successes %+v", good)
	}
	if good.Cached.P50 != 10*time.Millisecond {
		t.Fatalf("cached p50 %v", good.Cached.P50)
	}
	if good.Uncached.P50 != 40*time.Millisecond {
		t.Fatalf("uncached p50 %v", good.Uncached.P50)
	}
	if good.TLD.P50 != 80*time.Millisecond {
		t.Fatalf("tld p50 %v", good.TLD.P50)
	}
	if !good.NXChecked || good.NXRewrite {
		t.Fatalf("nx flags %+v", good)
	}
	if !good.DNSSECChecked || !good.DNSSECValidate {
		t.Fatalf("dnssec %+v", good)
	}
	if dead.Successes != 0 || dead.Attempts != 12 {
		t.Fatalf("dead %+v", dead)
	}
	if dead.NXChecked || dead.DNSSECChecked || dead.DNSSECValidate {
		t.Fatalf("timeout must leave NX/DNSSEC unchecked, got %+v", dead)
	}

	f.mu.Lock()
	names := append([]string(nil), f.names...)
	f.mu.Unlock()
	var unique, tldQ int
	for _, n := range names {
		if strings.Contains(n, "u-abc123-") {
			unique++
		}
		if strings.Contains(n, "c-abc123-") {
			tldQ++
		}
	}
	if unique < 4 {
		t.Fatalf("expected unique uncached qnames, got %v", names)
	}
	if tldQ < 4 {
		t.Fatalf("expected tld qnames, got %v", names)
	}
	if bench.UncachedName("abc123", 0, "example.com") != "u-abc123-0.example.com" {
		t.Fatal("uncached shape")
	}
	if bench.TLDName("abc123", 0) != "c-abc123-0.com" {
		t.Fatal("tld shape")
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
		CheckTLD:  true,
		RunID:     "zz",
		Query:     f.Query,
	}
	ms := bench.Run(context.Background(), cfg)
	if !ms[0].NXRewrite {
		t.Fatalf("expected rewrite: %+v", ms[0])
	}
}

func TestDNSSECNotValidating(t *testing.T) {
	f := &fakeDNS{
		uncachedRTT:    time.Millisecond,
		cachedRTT:      time.Millisecond,
		dnssecSERVFAIL: false,
	}
	cfg := bench.Config{
		Resolvers:   []resolvers.Resolver{{Name: "Plain", IP: "192.0.2.3", Port: "53"}},
		Domains:     []string{"example.com"},
		Queries:     1,
		CheckDNSSEC: true,
		RunID:       "ds",
		Query:       f.Query,
	}
	ms := bench.Run(context.Background(), cfg)
	if !ms[0].DNSSECChecked || ms[0].DNSSECValidate {
		t.Fatalf("expected non-validating: %+v", ms[0])
	}
}

func TestNXAndDNSSECTimeoutLeftUnchecked(t *testing.T) {
	f := &fakeDNS{dropAll: true}
	cfg := bench.Config{
		Resolvers:   []resolvers.Resolver{{Name: "Silent", IP: "192.0.2.8", Port: "53"}},
		Domains:     []string{"example.com"},
		Queries:     1,
		CheckNX:     true,
		CheckDNSSEC: true,
		RunID:       "to",
		Query:       f.Query,
	}
	ms := bench.Run(context.Background(), cfg)
	if ms[0].NXChecked {
		t.Fatalf("NX timeout marked checked: %+v", ms[0])
	}
	if ms[0].NXRewrite {
		t.Fatalf("NX timeout must not claim rewrite: %+v", ms[0])
	}
	if ms[0].DNSSECChecked {
		t.Fatalf("DNSSEC timeout marked checked: %+v", ms[0])
	}
	if ms[0].DNSSECValidate {
		t.Fatalf("DNSSEC timeout must not claim validating: %+v", ms[0])
	}
}

func TestUncachedAndTLDName(t *testing.T) {
	if got := bench.UncachedName("deadbeef", 3, "google.com"); got != "u-deadbeef-3.google.com" {
		t.Fatalf("got %s", got)
	}
	if got := bench.TLDName("deadbeef", 3); got != "c-deadbeef-3.com" {
		t.Fatalf("got %s", got)
	}
}
