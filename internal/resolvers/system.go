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

	var out []Resolver
	sc := bufio.NewScanner(f)
	n := 0
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.EqualFold(fields[0], "nameserver") {
			continue
		}
		ip := net.ParseIP(fields[1])
		if ip == nil || ip.To4() == nil {
			continue
		}
		n++
		out = append(out, Resolver{
			Name:   fmt.Sprintf("System-%d", n),
			IP:     ip.To4().String(),
			Port:   "53",
			System: true,
		})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// System returns OS-configured IPv4 resolvers when /etc/resolv.conf is present.
func System() ([]Resolver, error) {
	const path = "/etc/resolv.conf"
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return SystemFromResolvConf(path)
}
