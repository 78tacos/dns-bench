package resolvers

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

// SystemFromResolvConf reads nameserver lines from a resolv.conf-style file.
// IPv6 nameservers are skipped in v1.
func SystemFromResolvConf(path string) ([]Resolver, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var ips []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.EqualFold(fields[0], "nameserver") {
			continue
		}
		ips = append(ips, fields[1])
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return fromNameserverIPs(ips), nil
}

// fromNameserverIPs keeps unique IPv4 addresses and skips IPv6 / unspecified.
func fromNameserverIPs(ips []string) []Resolver {
	seen := make(map[string]struct{})
	var out []Resolver
	n := 0
	for _, s := range ips {
		ip := net.ParseIP(strings.TrimSpace(s))
		if ip == nil || ip.To4() == nil || ip.IsUnspecified() {
			continue
		}
		v4 := ip.To4().String()
		if _, ok := seen[v4]; ok {
			continue
		}
		seen[v4] = struct{}{}
		n++
		out = append(out, Resolver{
			Name:   fmt.Sprintf("System-%d", n),
			IP:     v4,
			Port:   "53",
			System: true,
		})
	}
	return out
}
