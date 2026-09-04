package snapshot

import (
	"strings"
	"testing"
)

func TestCaptureStoresTheDeltaBetweenExternalSnapshots(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := Capture("terminal-1", []byte("prompt\nfirst result\nprompt\n")); err != nil {
		t.Fatal(err)
	}
	if err := Capture("terminal-1", []byte("prompt\nfirst result\nprompt\nsecond result\nprompt\n")); err != nil {
		t.Fatal(err)
	}
	got, err := Last("terminal-1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != "second result" {
		t.Fatalf("Last() = %q", got)
	}
}

func TestLastStripsAChangedTrailingPrompt(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := Capture("terminal-1", []byte("old-prompt\n")); err != nil {
		t.Fatal(err)
	}
	if err := Capture("terminal-1", []byte("old-prompt\nbuild failed\nnew-prompt-with-error\n")); err != nil {
		t.Fatal(err)
	}
	got, err := Last("terminal-1")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "build failed" {
		t.Fatalf("Last() = %q", got)
	}
}

func TestLastNeedsTwoCommandBoundaries(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := Capture("terminal-1", []byte("prompt\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := Last("terminal-1"); err == nil || !strings.Contains(err.Error(), "capture the next command boundary") {
		t.Fatalf("Last() error = %v", err)
	}
}

func TestIDRejectsPaths(t *testing.T) {
	t.Setenv("ASK_CAPTURE_ID", "../another-session")
	if _, err := ID(); err == nil {
		t.Fatal("ID() accepted a path")
	}
}
