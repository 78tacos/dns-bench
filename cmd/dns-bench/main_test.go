package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"
)

func captureStderr(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	runErr := fn()
	_ = w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	_ = r.Close()
	return buf.String(), runErr
}

func TestHelpExitsClean(t *testing.T) {
	for _, arg := range []string{"-h", "-help"} {
		out, err := captureStderr(t, func() error { return run([]string{arg}) })
		if err != nil {
			t.Fatalf("%s: %v\n%s", arg, err, out)
		}
		if out == "" {
			t.Fatalf("%s: expected usage on stderr", arg)
		}
	}
}

func TestUnknownFlagDoesNotDoublePrintFromRun(t *testing.T) {
	out, err := captureStderr(t, func() error { return run([]string{"-not-a-real-flag"}) })
	if err == nil {
		t.Fatal("expected error")
	}
	var printed printedError
	if !errors.As(err, &printed) {
		t.Fatalf("want printedError, got %T %v", err, err)
	}
	if out == "" {
		t.Fatal("FlagSet should print usage")
	}
	if bytes.Count([]byte(out), []byte("flag provided but not defined")) > 1 {
		t.Fatalf("duplicated flag error:\n%s", out)
	}
}
