package resolvers_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/78tacos/dns-bench/internal/resolvers"
)

func TestParseAddress(t *testing.T) {
	ip, port, err := resolvers.ParseAddress("1.1.1.1")
	if err != nil || ip != "1.1.1.1" || port != "53" {
		t.Fatalf("plain: %s %s %v", ip, port, err)
	}
	ip, port, err = resolvers.ParseAddress("8.8.8.8:5353")
	if err != nil || ip != "8.8.8.8" || port != "5353" {
		t.Fatalf("port: %s %s %v", ip, port, err)
	}
	if _, _, err := resolvers.ParseAddress("::1"); err == nil {
		t.Fatal("expected IPv6 rejection")
	}
	if _, _, err := resolvers.ParseAddress("not-an-ip"); err == nil {
		t.Fatal("expected garbage rejection")
	}
	if _, _, err := resolvers.ParseAddress(""); err == nil {
		t.Fatal("expected empty rejection")
	}
	if _, _, err := resolvers.ParseAddress("2001:4860:4860::8888"); err == nil {
		t.Fatal("expected IPv6 rejection")
	}
}

func TestCustomAndMerge(t *testing.T) {
	extra, err := resolvers.Custom([]string{"1.1.1.1", "192.0.2.1:5353"})
	if err != nil {
		t.Fatal(err)
	}
	if extra[0].Name != "Custom-1" || extra[1].Port != "5353" {
		t.Fatalf("custom: %+v", extra)
	}
	pub := resolvers.PublicIPv4()
	sys := []resolvers.Resolver{{Name: "System-1", IP: "1.1.1.1", Port: "53", System: true}}
	merged := resolvers.Merge(sys, extra, pub)
	if merged[0].IP != "1.1.1.1" || !merged[0].System {
		t.Fatalf("system should lead and mark 1.1.1.1: %+v", merged[0])
	}
	if !containsAddr(merged, "192.0.2.1:5353") {
		t.Fatal("missing custom port")
	}
	// 1.1.1.1 must appear once
	n := 0
	for _, r := range merged {
		if r.IP == "1.1.1.1" && r.Port == "53" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("dup 1.1.1.1 count %d", n)
	}
}

func TestPublicListHasExpectedProviders(t *testing.T) {
	want := map[string]string{
		"1.1.1.1":        "Cloudflare",
		"8.8.8.8":        "Google",
		"9.9.9.9":        "Quad9",
		"208.67.222.222": "OpenDNS",
	}
	got := map[string]string{}
	for _, r := range resolvers.PublicIPv4() {
		got[r.IP] = r.Name
		if r.Label() == "" || r.Addr() == "" {
			t.Fatalf("empty label/addr %+v", r)
		}
	}
	for ip, name := range want {
		if got[ip] != name {
			t.Fatalf("%s: got %q want %q", ip, got[ip], name)
		}
	}
}

func TestSystemFromResolvConf(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "resolv.conf")
	body := `# generated
nameserver 192.0.2.53
nameserver 2001:db8::1
nameserver 198.51.100.53
search example.test
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	rs, err := resolvers.SystemFromResolvConf(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 2 {
		t.Fatalf("len %d (IPv6 should be skipped)", len(rs))
	}
	if rs[0].IP != "192.0.2.53" || !rs[0].System || rs[0].Name != "System-1" {
		t.Fatalf("first: %+v", rs[0])
	}
	if rs[1].IP != "198.51.100.53" {
		t.Fatalf("second: %+v", rs[1])
	}
}

func containsAddr(rs []resolvers.Resolver, addr string) bool {
	for _, r := range rs {
		if r.Addr() == addr {
			return true
		}
	}
	return false
}
