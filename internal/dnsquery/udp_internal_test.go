package dnsquery

import (
	"testing"
	"time"
)

func TestClampRTT(t *testing.T) {
	if got := clampRTT(0); got != time.Nanosecond {
		t.Fatalf("zero: got %v", got)
	}
	if got := clampRTT(-time.Millisecond); got != time.Nanosecond {
		t.Fatalf("negative: got %v", got)
	}
	if got := clampRTT(5 * time.Millisecond); got != 5*time.Millisecond {
		t.Fatalf("positive: got %v", got)
	}
}
