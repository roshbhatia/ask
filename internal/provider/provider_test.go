package provider

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	shared "github.com/roshbhatia/go-utils/provider"
	"go.yaml.in/yaml/v3"
)

func helperManifest(t *testing.T, directory, name, mode string) {
	t.Helper()
	manifest := shared.Manifest{
		Version:     Protocol,
		Name:        name,
		Description: "test provider",
		Command:     []string{os.Args[0], "-test.run=TestProviderProcess", "--"},
		Actions: map[string]shared.Action{
			ActionGenerate: {Description: "generate", Env: map[string]string{"ASK_HELPER": mode}},
			ActionModels:   {Description: "models", Env: map[string]string{"ASK_HELPER": "models"}},
		},
	}
	raw, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "provider.yaml"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestProviderProcess(t *testing.T) {
	mode := os.Getenv("ASK_HELPER")
	if mode == "" {
		return
	}
	var envelope Envelope
	if err := json.NewDecoder(os.Stdin).Decode(&envelope); err != nil {
		os.Exit(2)
	}
	encoder := json.NewEncoder(os.Stdout)
	switch mode {
	case "models":
		_ = encoder.Encode(modelResponse{Version: Protocol, Models: []string{"small", "large"}})
	case "invalid":
		_, _ = os.Stdout.WriteString("not json\n")
	default:
		_ = encoder.Encode(Event{Version: Protocol, Kind: Started, Text: envelope.Request.Model})
		_ = encoder.Encode(Event{Version: Protocol, Kind: Text, Text: "answer"})
		_ = encoder.Encode(Event{Version: Protocol, Kind: Done, Result: &Result{Text: "answer"}})
	}
	os.Exit(0)
}

func TestDiscoverAndRunExternalProvider(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("ASK_PROVIDER_PATH", "")
	directory := filepath.Join(root, "ask", "providers", "sample")
	helperManifest(t, directory, "sample", "ok")

	known, err := Discover()
	if err != nil {
		t.Fatal(err)
	}
	if len(known) != 1 || known[0].Name != "sample" || !known[0].Ready() {
		t.Fatalf("got %#v", known)
	}
	if models := known[0].Models(); len(models) != 2 || models[0] != "small" {
		t.Fatalf("got models %#v", models)
	}

	agent, err := Find("sample")
	if err != nil {
		t.Fatal(err)
	}
	events, err := agent.Run(context.Background(), Request{Prompt: "question", Model: "small", Dir: root})
	if err != nil {
		t.Fatal(err)
	}
	var result *Result
	for event := range events {
		if event.Kind == Done {
			result = event.Result
		}
	}
	if result == nil || result.Failed || result.Text != "answer" {
		t.Fatalf("got %#v", result)
	}
}

func TestExternalProviderRejectsMalformedProtocol(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("ASK_PROVIDER_PATH", "")
	helperManifest(t, filepath.Join(root, "ask", "providers", "broken"), "broken", "invalid")
	agent, err := Find("broken")
	if err != nil {
		t.Fatal(err)
	}
	events, err := agent.Run(context.Background(), Request{Dir: root})
	if err != nil {
		t.Fatal(err)
	}
	var result *Result
	for event := range events {
		if event.Kind == Done {
			result = event.Result
		}
	}
	if result == nil || !result.Failed || !strings.Contains(result.Reason, "invalid JSON") {
		t.Fatalf("got %#v", result)
	}
}

func TestUserProviderWinsOverPackagedProvider(t *testing.T) {
	root := t.TempDir()
	user := filepath.Join(root, "ask", "providers", "sample")
	packaged := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("ASK_PROVIDER_PATH", packaged)
	helperManifest(t, user, "sample", "ok")
	helperManifest(t, packaged, "sample", "invalid")

	loaded, err := Loaded()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || filepath.Dir(loaded[0].Path) != user {
		t.Fatalf("got %#v", loaded)
	}
}

func TestCompatibilityAliases(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("ASK_PROVIDER_PATH", "")
	helperManifest(t, filepath.Join(root, "ask", "providers", "claude"), "claude", "ok")
	info, found := Lookup("cld")
	if !found || info.Name != "claude" {
		t.Fatalf("got %#v, %v", info, found)
	}
}
