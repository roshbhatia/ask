package schema

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestCompletePathsPreservesSchemaPrefix(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "result.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(directory, "reports"), 0o700); err != nil {
		t.Fatal(err)
	}
	prefix := "@" + directory + string(filepath.Separator) + "re"
	want := []string{
		"@" + filepath.Join(directory, "reports") + string(filepath.Separator),
		"@" + filepath.Join(directory, "result.json"),
	}
	if got := CompletePaths(prefix); !slices.Equal(got, want) {
		t.Fatalf("CompletePaths(%q) = %#v, want %#v", prefix, got, want)
	}
}
