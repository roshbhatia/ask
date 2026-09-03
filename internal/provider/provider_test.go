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
			ActionValidate: {Description: "validate", Env: map[string]string{"ASK_HELPER": "validate"}},
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
		_ = encoder.Encode(ModelResponse{Version: Protocol, Models: []string{"small", "large"}})
	case "validate":
		_ = encoder.Encode(ValidationResponse{Version: Protocol, Status: "ok"})
	case "invalid":
		_, _ = os.Stdout.WriteString("not json\n")
	case "exit-after-result":
		_ = encoder.Encode(Event{Version: Protocol, Kind: Done, Result: &Result{Text: "not a success"}})
		os.Exit(2)
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

func TestExternalProviderRejectsNonzeroExitAfterResult(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("ASK_PROVIDER_PATH", "")
	helperManifest(t, filepath.Join(root, "ask", "providers", "broken"), "broken", "exit-after-result")
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
	if result == nil || !result.Failed {
		t.Fatalf("got %#v", result)
	}
}

func TestValidateExercisesProviderContract(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("ASK_PROVIDER_PATH", "")
	helperManifest(t, filepath.Join(root, "ask", "providers", "sample"), "sample", "ok")
	reports, err := Validate("sample", root)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || !reports[0].OK() {
		t.Fatalf("reports = %#v", reports)
	}
	found := false
	for _, check := range reports[0].Checks {
		if check.Kind == "contract" && check.Status == shared.CheckOK {
			found = true
		}
	}
	if !found {
		t.Fatalf("contract check missing from %#v", reports[0])
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

func TestDiscoverUsesXDGDataProviders(t *testing.T) {
	configHome := t.TempDir()
	dataHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_DATA_DIRS", "")
	t.Setenv("ASK_PROVIDER_PATH", "")
	helperManifest(t, filepath.Join(dataHome, "ask", "providers", "sample"), "sample", "ok")

	known, err := Discover()
	if err != nil {
		t.Fatal(err)
	}
	if len(known) != 1 || known[0].Name != "sample" {
		t.Fatalf("got %#v", known)
	}
}

func TestXDGDataHomeWinsOverSystemData(t *testing.T) {
	configHome := t.TempDir()
	dataHome := t.TempDir()
	dataDirectory := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_DATA_DIRS", dataDirectory)
	t.Setenv("ASK_PROVIDER_PATH", "")
	user := filepath.Join(dataHome, "ask", "providers", "sample")
	system := filepath.Join(dataDirectory, "ask", "providers", "sample")
	helperManifest(t, user, "sample", "ok")
	helperManifest(t, system, "sample", "invalid")

	loaded, err := Loaded()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || filepath.Dir(loaded[0].Path) != user {
		t.Fatalf("got %#v", loaded)
	}
}

func TestDiscoverWorksWithoutProviders(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_DATA_DIRS", t.TempDir())
	t.Setenv("ASK_PROVIDER_PATH", "")

	known, err := Discover()
	if err != nil {
		t.Fatal(err)
	}
	if len(known) != 0 {
		t.Fatalf("got %#v", known)
	}
}

func TestDiscoveryReportsMalformedManifest(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_DATA_DIRS", t.TempDir())
	t.Setenv("ASK_PROVIDER_PATH", "")
	directory := filepath.Join(root, "ask", "providers", "broken")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "provider.yaml"), []byte("not: a: manifest\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Known(); err == nil || !strings.Contains(err.Error(), "provider.yaml") {
		t.Fatalf("Known() error = %v", err)
	}
}

func TestSendEventStopsWhenCanceled(t *testing.T) {
	events := make(chan Event, 1)
	events <- Event{Kind: Started}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if sendEvent(ctx, events, Event{Kind: Text}) {
		t.Fatal("sendEvent reported a send after cancellation")
	}
}

func TestAskSchemaRequiresGenerateAndValidationActions(t *testing.T) {
	raw, err := Schema()
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	properties := document["properties"].(map[string]any)
	actions := properties["actions"].(map[string]any)
	required := actions["required"].([]any)
	if len(required) != 2 || required[0] != ActionGenerate || required[1] != ActionValidate {
		t.Fatalf("required actions = %#v", required)
	}
}

func TestWireRequestSchemaUsesTextInput(t *testing.T) {
	schemas, err := WireSchemas()
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(schemas["protocol.request.schema.json"], &document); err != nil {
		t.Fatal(err)
	}
	definitions := document["$defs"].(map[string]any)
	request := definitions["Request"].(map[string]any)
	properties := request["properties"].(map[string]any)
	input := properties["input"].(map[string]any)
	if input["type"] != "string" {
		t.Fatalf("input schema = %#v", input)
	}
}
