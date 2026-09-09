package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDemoTextDoesNotChangePackageMetadata(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll("extras/local", 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"extras/local/package.json":  `{"name":"local","core":"ask","brew":"ask-provider-local","binary":"ask-provider-local","manifest_path":"share/ask/providers/local/provider.yaml"}`,
		"extras/local/provider.yaml": "description: Answer questions through a local runtime\n",
		"extras/local/demo.json":     `{"name":"local","summary":"Review a release fix","status":"pending"}`,
		"extras/local/demo.sh":       "",
		"extras/local/demo.tape":     "",
		"extras/README.md":           "<!-- BEGIN GENERATED CATALOG -->\n<!-- END GENERATED CATALOG -->\n",
	}
	for path, text := range files {
		if err := os.WriteFile(path, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
	oldArgs := os.Args
	os.Args = []string{"docgen"}
	t.Cleanup(func() { os.Args = oldArgs })
	if err := run(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile("package-index.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("extras/local/demo.json", []byte(`{"name":"local","summary":"Inspect a migration","status":"pending"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := run(); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile("package-index.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("demo text changed the package contract")
	}
	var index struct {
		Packages []Archive `json:"packages"`
	}
	if err := json.Unmarshal(after, &index); err != nil {
		t.Fatal(err)
	}
	if len(index.Packages) != 1 || index.Packages[0].Description != "Answer questions through a local runtime" {
		t.Fatalf("package capability = %v", index.Packages)
	}
	readme, err := os.ReadFile(filepath.Join("extras", "local", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(readme, []byte("Inspect a migration")) || bytes.Contains(readme, []byte("demo.gif")) {
		t.Fatalf("pending demo documentation = %s", readme)
	}
}
