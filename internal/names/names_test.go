package names_test

import (
	"testing"

	"github.com/78tacos/dns-bench/internal/names"
)

func TestTake(t *testing.T) {
	all := names.Take(0)
	if len(all) != len(names.Popular) {
		t.Fatalf("full list %d", len(all))
	}
	got := names.Take(3)
	if len(got) != 3 || got[0] != names.Popular[0] {
		t.Fatalf("%v", got)
	}
	got[0] = "mutated"
	if names.Popular[0] == "mutated" {
		t.Fatal("Take must copy")
	}
}
