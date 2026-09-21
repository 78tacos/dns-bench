package resolvers

import (
	"fmt"
	"net"
	"strings"
)

// Resolver is an IPv4 DNS server to probe.
type Resolver struct {
	Name   string
	IP     string
	Port   string
	System bool
}

// Addr returns host:port for UDP queries.
func (r Resolver) Addr() string {
	port := r.Port
	if port == "" {
		port = "53"
	}
	return net.JoinHostPort(r.IP, port)
}

// Label is the human-readable table name.
func (r Resolver) Label() string {
	port := r.Port
	if port == "" {
		port = "53"
	}
	if port != "53" {
		return fmt.Sprintf("%s %s:%s", r.Name, r.IP, port)
	}
	return fmt.Sprintf("%s %s", r.Name, r.IP)
}

// PublicIPv4 is a curated, original list of well-known public resolvers.
// Addresses are public anycast documentation, not a third-party product dump.
func PublicIPv4() []Resolver {
	return []Resolver{
		{Name: "Cloudflare", IP: "1.1.1.1", Port: "53"},
		{Name: "Cloudflare", IP: "1.0.0.1", Port: "53"},
		{Name: "Google", IP: "8.8.8.8", Port: "53"},
		{Name: "Google", IP: "8.8.4.4", Port: "53"},
		{Name: "Quad9", IP: "9.9.9.9", Port: "53"},
		{Name: "Quad9", IP: "149.112.112.112", Port: "53"},
		{Name: "OpenDNS", IP: "208.67.222.222", Port: "53"},
		{Name: "OpenDNS", IP: "208.67.220.220", Port: "53"},
		{Name: "NextDNS", IP: "45.90.28.0", Port: "53"},
		{Name: "NextDNS", IP: "45.90.30.0", Port: "53"},
		{Name: "AdGuard", IP: "94.140.14.14", Port: "53"},
		{Name: "AdGuard", IP: "94.140.15.15", Port: "53"},
		{Name: "Control D", IP: "76.76.2.0", Port: "53"},
		{Name: "Control D", IP: "76.76.10.0", Port: "53"},
		{Name: "DNS.SB", IP: "185.222.222.222", Port: "53"},
		{Name: "DNS.SB", IP: "185.184.222.222", Port: "53"},
		{Name: "CleanBrowsing", IP: "185.228.168.9", Port: "53"},
		{Name: "CleanBrowsing", IP: "185.228.169.9", Port: "53"},
		{Name: "Mullvad", IP: "194.242.2.2", Port: "53"},
		{Name: "Verisign", IP: "64.6.64.6", Port: "53"},
		{Name: "Verisign", IP: "64.6.65.6", Port: "53"},
		{Name: "Hurricane Electric", IP: "74.82.42.42", Port: "53"},
		{Name: "Yandex", IP: "77.88.8.8", Port: "53"},
		{Name: "Yandex", IP: "77.88.8.1", Port: "53"},
		{Name: "Comodo", IP: "8.26.56.26", Port: "53"},
		{Name: "Comodo", IP: "8.20.247.20", Port: "53"},
	}
}

// ParseAddress accepts IPv4 or IPv4:port. IPv6 is rejected in v1.
func ParseAddress(s string) (ip, port string, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", "", fmt.Errorf("empty resolver address")
	}
	host, p, splitErr := net.SplitHostPort(s)
	if splitErr != nil {
		host, p = s, "53"
	}
	parsed := net.ParseIP(host)
	if parsed == nil {
		return "", "", fmt.Errorf("invalid IP address %q", s)
	}
	if parsed.To4() == nil {
		return "", "", fmt.Errorf("IPv6 not supported in v1 (got %q)", s)
	}
	if p == "" {
		p = "53"
	}
	return parsed.To4().String(), p, nil
}

// Custom builds resolvers from user-supplied addresses.
func Custom(addrs []string) ([]Resolver, error) {
	out := make([]Resolver, 0, len(addrs))
	for i, a := range addrs {
		ip, port, err := ParseAddress(a)
		if err != nil {
			return nil, err
		}
		out = append(out, Resolver{
			Name: fmt.Sprintf("Custom-%d", i+1),
			IP:   ip,
			Port: port,
		})
	}
	return out, nil
}

// Merge de-duplicates by host:port. System and custom entries win on name;
// a later duplicate of a public IP is skipped but may set System.
func Merge(groups ...[]Resolver) []Resolver {
	seen := make(map[string]int)
	out := make([]Resolver, 0)
	for _, group := range groups {
		for _, r := range group {
			if r.Port == "" {
				r.Port = "53"
			}
			key := r.Addr()
			if idx, ok := seen[key]; ok {
				if r.System {
					out[idx].System = true
					if !strings.Contains(strings.ToLower(out[idx].Name), "system") {
						out[idx].Name = "System / " + out[idx].Name
					}
				}
				continue
			}
			seen[key] = len(out)
			out = append(out, r)
		}
	}
	return out
}
