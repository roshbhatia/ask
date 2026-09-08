package ui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/roshbhatia/go-utils/cell"
	shared "github.com/roshbhatia/go-utils/keymap"
)

func TestSplitFitsTerminalCells(t *testing.T) {
	t.Parallel()

	got := split("\x1b[31m界界界 status\x1b[0m", "1:23", 12)
	if width := cell.Width(got); width != 12 {
		t.Fatalf("split width = %d, want 12: %q", width, got)
	}
	if !strings.HasSuffix(got, "1:23") {
		t.Fatalf("split lost the right edge: %q", got)
	}
}

func TestFrameFitsStyledWideRows(t *testing.T) {
	t.Parallel()

	got := (frame{title: "ask", width: 8, rows: []string{"\x1b[31m界界界界界\x1b[0m"}}).String()
	for index, line := range strings.Split(got, "\n") {
		if width := cell.Width(line); width != 12 {
			t.Fatalf("line %d width = %d, want 12: %q", index, width, line)
		}
	}
}

func TestHelpComesFromSharedCatalog(t *testing.T) {
	t.Parallel()

	rows, err := keys.HelpRows(shared.Context{"pick"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 || rows[0].ID != "pick-up" || rows[4].ID != "pick-quit" {
		t.Fatalf("picker help rows = %#v", rows)
	}
	help := bubbleBinding("pick-take").Help()
	if help.Key != "enter" || help.Desc != "run it" {
		t.Fatalf("Bubble Tea help = %#v", help)
	}
}

func TestThemePathIgnoresRelativeXDGRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "relative")

	want := filepath.Join(home, ".config", themeFile)
	if got := themePath(); got != want {
		t.Fatalf("themePath() = %q, want %q", got, want)
	}
}
