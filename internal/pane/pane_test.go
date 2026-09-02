package pane

import (
	"slices"
	"testing"
)

func TestPaneCaptureDoesNotStartAHeadlessMux(t *testing.T) {
	want := []string{"cli", "--no-auto-start", "get-text", "--start-line", scrollback}
	if got := weztermArgs(); !slices.Equal(got, want) {
		t.Fatalf("weztermArgs() = %q, want %q", got, want)
	}
}

func TestPaneCaptureHasNoControllingTerminal(t *testing.T) {
	command := weztermCommand("wezterm")
	if command.SysProcAttr == nil || !command.SysProcAttr.Setsid {
		t.Fatal("wezterm capture can write terminal replies into the shell input stream")
	}
}
