package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/78tacos/dns-bench/internal/bench"
	"github.com/78tacos/dns-bench/internal/names"
	"github.com/78tacos/dns-bench/internal/rank"
	"github.com/78tacos/dns-bench/internal/report"
	"github.com/78tacos/dns-bench/internal/resolvers"
	"github.com/78tacos/dns-bench/internal/version"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		var printed printedError
		if errors.As(err, &printed) {
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "dns-bench: %v\n", err)
		os.Exit(1)
	}
}

// printedError is a FlagSet parse failure already written to stderr.
type printedError struct{ err error }

func (e printedError) Error() string { return e.err.Error() }
func (e printedError) Unwrap() error { return e.err }

func run(args []string) error {
	fs := flag.NewFlagSet("dns-bench", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	queries := fs.Int("queries", 8, "timed queries per resolver per latency phase (cached, uncached, tld)")
	timeout := fs.Duration("timeout", 2*time.Second, "per-query timeout")
	rankMode := fs.String("rank", "blended", "ranking: blended, cached, uncached, tld, reliability")
	checkNX := fs.Bool("nxdomain", true, "probe NXDOMAIN rewrite (disable with -nxdomain=false)")
	checkTLD := fs.Bool("tld", true, "time .com TLD-path queries (disable with -tld=false)")
	checkDNSSEC := fs.Bool("dnssec", false, "probe DNSSEC validation via dnssec-failed.org")
	noSystem := fs.Bool("no-system", false, "skip the OS-configured resolver from resolv.conf")
	noPublic := fs.Bool("no-public", false, "skip the built-in public resolver list")
	listOnly := fs.Bool("list", false, "print probe list and exit")
	showVersion := fs.Bool("version", false, "print version and exit")
	quiet := fs.Bool("quiet", false, "skip banner; still print the table")
	jsonPath := fs.String("json", "", "write JSON report to this path")
	csvPath := fs.String("csv", "", "write CSV report to this path")
	htmlPath := fs.String("html", "", "write HTML report to this path")
	resolversCSV := fs.String("resolvers", "", "comma-separated extra IPv4 addresses (optional :port)")

	var extra []string
	fs.Func("resolver", "additional IPv4 resolver (repeatable, optional :port)", func(s string) error {
		extra = append(extra, s)
		return nil
	})

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return printedError{err}
	}
	if *showVersion {
		fmt.Printf("dns-bench %s\n", version.Version)
		return nil
	}

	mode, err := rank.ParseMode(*rankMode)
	if err != nil {
		return err
	}

	if *resolversCSV != "" {
		for _, p := range strings.Split(*resolversCSV, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				extra = append(extra, p)
			}
		}
	}

	custom, err := resolvers.Custom(extra)
	if err != nil {
		return err
	}

	var system []resolvers.Resolver
	if !*noSystem {
		system, err = resolvers.System()
		if err != nil {
			fmt.Fprintf(os.Stderr, "dns-bench: warning: system resolver: %v\n", err)
			system = nil
		}
	}

	var public []resolvers.Resolver
	if !*noPublic {
		public = resolvers.PublicIPv4()
	}

	probes := resolvers.Merge(system, custom, public)
	if len(probes) == 0 {
		return fmt.Errorf("no resolvers to probe (try -resolver 1.1.1.1)")
	}

	if *listOnly {
		for _, r := range probes {
			mark := ""
			if r.System {
				mark = " [system]"
			}
			fmt.Printf("%s\t%s%s\n", r.Name, r.Addr(), mark)
		}
		return nil
	}

	if !*quiet {
		fmt.Fprintf(os.Stderr, "dns-bench v%s — resolver lab for your network edge\n", version.Version)
		fmt.Fprintf(os.Stderr, "%s\n\n", report.Disclaimer)
		fmt.Fprintf(os.Stderr, "Probing %d resolver(s), %d timed queries × latency phases (timeout %s). Live bench needs UDP/53.\n",
			len(probes), *queries, timeout)
	}

	var mu sync.Mutex
	done := 0
	cfg := bench.Config{
		Resolvers:   probes,
		Domains:     names.Take(*queries),
		Queries:     *queries,
		Timeout:     *timeout,
		CheckNX:     *checkNX,
		CheckTLD:    *checkTLD,
		CheckDNSSEC: *checkDNSSEC,
		RunID:       newRunID(),
		OnDone: func(m bench.Measurement) {
			mu.Lock()
			defer mu.Unlock()
			done++
			if *quiet {
				return
			}
			fmt.Fprintf(os.Stderr, "[%d/%d] %s  cached p50=%s  uncached p50=%s  tld p50=%s  loss=%.0f%%\n",
				done, len(probes), m.Resolver.Label(),
				msOrDash(m.Cached.P50, m.Cached.Count),
				msOrDash(m.Uncached.P50, m.Uncached.Count),
				msOrDash(m.TLD.P50, m.TLD.Count),
				lossPct(m.Successes, m.Attempts),
			)
		},
	}

	ctx := context.Background()
	measurements := bench.Run(ctx, cfg)
	inputs := make([]rank.Input, len(measurements))
	for i, m := range measurements {
		inputs[i] = rank.Input{
			Name:           m.Resolver.Name,
			Address:        m.Resolver.Addr(),
			System:         m.Resolver.System,
			Cached:         m.Cached,
			Uncached:       m.Uncached,
			TLD:            m.TLD,
			Successes:      m.Successes,
			Attempts:       m.Attempts,
			NXRewrite:      m.NXRewrite,
			NXChecked:      m.NXChecked,
			DNSSECChecked:  m.DNSSECChecked,
			DNSSECValidate: m.DNSSECValidate,
		}
	}
	rows := rank.Apply(inputs, mode)

	fmt.Print(report.Table(rows))
	fmt.Println()
	fmt.Println(report.SummaryLine(rows, mode))
	fmt.Println("* = OS-configured resolver")

	now := time.Now()
	if *jsonPath != "" {
		raw, err := report.JSON(rows, mode, now)
		if err != nil {
			return err
		}
		if err := os.WriteFile(*jsonPath, raw, 0o644); err != nil {
			return err
		}
	}
	if *csvPath != "" {
		f, err := os.Create(*csvPath)
		if err != nil {
			return err
		}
		err = report.CSV(f, rows)
		cerr := f.Close()
		if err != nil {
			return err
		}
		if cerr != nil {
			return cerr
		}
	}
	if *htmlPath != "" {
		if err := os.WriteFile(*htmlPath, report.HTML(rows, mode, now), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func newRunID() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func msOrDash(d time.Duration, count int) string {
	if count == 0 {
		return "—"
	}
	return fmt.Sprintf("%.1fms", float64(d)/float64(time.Millisecond))
}

func lossPct(successes, attempts int) float64 {
	if attempts <= 0 {
		return 100
	}
	return (1 - float64(successes)/float64(attempts)) * 100
}
