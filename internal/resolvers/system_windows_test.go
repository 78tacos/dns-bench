//go:build windows

package resolvers

import "testing"

func TestSystemWindows(t *testing.T) {
	rs, err := System()
	if err != nil {
		t.Fatalf("System: %v", err)
	}
	for _, r := range rs {
		if r.IP == "" || r.Port != "53" || !r.System {
			t.Fatalf("unexpected resolver: %+v", r)
		}
	}
}

func TestCString(t *testing.T) {
	if got := cString([]byte{'1', '.', '1', '.', '1', '.', '1', 0, 'x'}); got != "1.1.1.1" {
		t.Fatalf("got %q", got)
	}
	if got := cString([]byte{'a', 'b'}); got != "ab" {
		t.Fatalf("no-nul: %q", got)
	}
	if got := cString(nil); got != "" {
		t.Fatalf("nil: %q", got)
	}
}
