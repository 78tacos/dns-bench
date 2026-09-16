package resolvers

import "testing"

func TestFromNameserverIPs(t *testing.T) {
	got := fromNameserverIPs([]string{
		"1.1.1.1",
		"::1",
		"0.0.0.0",
		" 8.8.8.8 ",
		"not-an-ip",
		"1.1.1.1",
		"2001:db8::1",
	})
	if len(got) != 2 {
		t.Fatalf("len=%d want 2: %+v", len(got), got)
	}
	if got[0].IP != "1.1.1.1" || got[0].Name != "System-1" || !got[0].System || got[0].Port != "53" {
		t.Fatalf("first: %+v", got[0])
	}
	if got[1].IP != "8.8.8.8" || got[1].Name != "System-2" {
		t.Fatalf("second: %+v", got[1])
	}
}
