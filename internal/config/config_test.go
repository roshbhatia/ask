package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadYAMLAndEnvironment(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	path := filepath.Join(root, "ask", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("version: ask.config/v1\nprovider:\n  default: disk\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ASK_PROVIDER_DEFAULT", "environment")
	value, err := Get(ProviderDefault)
	if err != nil {
		t.Fatal(err)
	}
	if value != "environment" {
		t.Fatalf("got %q", value)
	}
}

func TestLoadLegacyJSONWhenYAMLIsAbsent(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	path := filepath.Join(root, "ask", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"provider.default":"claude"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	value, err := Get(ProviderDefault)
	if err != nil {
		t.Fatal(err)
	}
	if value != "claude" {
		t.Fatalf("got %q", value)
	}
}

func TestConfigSchemaNamesVersionAndProvider(t *testing.T) {
	raw, err := Schema()
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{"ask.config/v1", "provider", "default"} {
		if !strings.Contains(text, want) {
			t.Fatalf("schema does not contain %q", want)
		}
	}
}
