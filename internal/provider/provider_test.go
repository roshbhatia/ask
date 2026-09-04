package provider

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
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

func relativeProviderManifest(t *testing.T, directory string) {
	t.Helper()
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	adapter := filepath.Join(directory, "adapter")
	script := `#!/bin/sh
payload=$(cat)
case "$payload" in
  *'"action":"inference.models"'*) printf '%s\n' '{"version":"provider/v1","models":["relative"]}' ;;
  *'"action":"provider.validate"'*) printf '%s\n' '{"version":"provider/v1","status":"ok"}' ;;
  *) printf '%s\n' '{"version":"provider/v1","type":"result","result":{"text":"relative answer"}}' ;;
esac
`
	if err := os.WriteFile(adapter, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := shared.Manifest{
		Version:     Protocol,
		Name:        "relative",
		Description: "relative command provider",
		Command:     []string{"./adapter"},
		Actions: map[string]shared.Action{
			ActionGenerate: {Description: "generate"},
			ActionModels:   {Description: "models"},
			ActionValidate: {Description: "validate"},
		},
	}
	raw, err := yaml.Marshal(manifest)
	if err != nil {
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

func TestRelativeProviderCommandResolvesFromManifestDirectory(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_DATA_DIRS", t.TempDir())
	t.Setenv("ASK_PROVIDER_PATH", "")
	directory := filepath.Join(root, "ask", "providers", "relative")
	relativeProviderManifest(t, directory)
	workingDirectory := t.TempDir()

	known, err := Discover()
	if err != nil {
		t.Fatal(err)
	}
	if len(known) != 1 || !known[0].Ready() {
		t.Fatalf("providers = %#v", known)
	}
	if models := known[0].Models(); !slices.Equal(models, []string{"relative"}) {
		t.Fatalf("models = %#v", models)
	}

	agent, err := Find("relative")
	if err != nil {
		t.Fatal(err)
	}
	events, err := agent.Run(context.Background(), Request{Dir: workingDirectory})
	if err != nil {
		t.Fatal(err)
	}
	var result *Result
	for event := range events {
		if event.Kind == Done {
			result = event.Result
		}
	}
	if result == nil || result.Failed || result.Text != "relative answer" {
		t.Fatalf("result = %#v", result)
	}

	reports, err := Validate("relative", workingDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || !reports[0].OK() {
		t.Fatalf("reports = %#v", reports)
	}
}

func TestDiscoverFollowsSymlinkedProviderDirectory(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_DATA_DIRS", t.TempDir())
	t.Setenv("ASK_PROVIDER_PATH", "")
	target := filepath.Join(t.TempDir(), "sample")
	helperManifest(t, target, "sample", "ok")
	providers := filepath.Join(root, "ask", "providers")
	if err := os.MkdirAll(providers, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(providers, "sample")); err != nil {
		t.Fatal(err)
	}

	known, err := Discover()
	if err != nil {
		t.Fatal(err)
	}
	if len(known) != 1 || known[0].Name != "sample" || !known[0].Ready() {
		t.Fatalf("providers = %#v", known)
	}
}

func TestValidationDiagnosesBrokenProviderDirectorySymlink(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_DATA_DIRS", t.TempDir())
	t.Setenv("ASK_PROVIDER_PATH", "")
	providers := filepath.Join(root, "ask", "providers")
	helperManifest(t, filepath.Join(providers, "sample"), "sample", "ok")
	broken := filepath.Join(providers, "broken")
	if err := os.Symlink(filepath.Join(root, "missing"), broken); err != nil {
		t.Fatal(err)
	}

	known, err := Discover()
	if err != nil {
		t.Fatal(err)
	}
	if len(known) != 1 || known[0].Name != "sample" {
		t.Fatalf("providers = %#v", known)
	}
	reports, err := Validate("", root)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 2 || reports[0].Provider != "sample" || !reports[0].OK() {
		t.Fatalf("reports = %#v", reports)
	}
	diagnostic := reports[1]
	if diagnostic.Provider != "broken" || diagnostic.OK() || len(diagnostic.Checks) != 1 {
		t.Fatalf("diagnostic = %#v", diagnostic)
	}
	if diagnostic.Checks[0].Target != broken || !strings.Contains(diagnostic.Checks[0].Message, "no such file") {
		t.Fatalf("check = %#v", diagnostic.Checks[0])
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

func TestNamedValidationIgnoresMalformedSibling(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_DATA_DIRS", t.TempDir())
	t.Setenv("ASK_PROVIDER_PATH", "")
	providers := filepath.Join(root, "ask", "providers")
	helperManifest(t, filepath.Join(providers, "sample"), "sample", "ok")
	broken := filepath.Join(providers, "broken")
	if err := os.MkdirAll(broken, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, "provider.yaml"), []byte("not: a: manifest\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	reports, err := Validate("sample", root)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || !reports[0].OK() {
		t.Fatalf("reports = %#v", reports)
	}
	all, err := Validate("", root)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[1].Provider != "broken" || all[1].OK() {
		t.Fatalf("full validation reports = %#v", all)
	}
}

func TestNamedValidationIgnoresMalformedFileBesideValidManifest(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_DATA_DIRS", t.TempDir())
	t.Setenv("ASK_PROVIDER_PATH", "")
	directory := filepath.Join(root, "ask", "providers", "sample")
	helperManifest(t, directory, "sample", "ok")
	if err := os.WriteFile(filepath.Join(directory, "broken.yaml"), []byte("not: a: manifest\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	reports, err := Validate("sample", root)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || !reports[0].OK() {
		t.Fatalf("reports = %#v", reports)
	}
}

func TestInvalidProviderRootDoesNotDisableDiscovery(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "config")
	valid := filepath.Join(root, "valid")
	invalid := filepath.Join(root, "not-a-directory")
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_DATA_DIRS", filepath.Join(root, "system-data"))
	t.Setenv("ASK_PROVIDER_PATH", strings.Join([]string{invalid, valid}, string(os.PathListSeparator)))
	if err := os.WriteFile(invalid, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	helperManifest(t, filepath.Join(valid, "sample"), "sample", "ok")

	known, err := Discover()
	if err != nil {
		t.Fatal(err)
	}
	if len(known) != 1 || known[0].Name != "sample" {
		t.Fatalf("providers = %#v", known)
	}
	reports, err := Validate("", root)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 2 || reports[0].Provider != "sample" || !reports[0].OK() || reports[1].OK() {
		t.Fatalf("reports = %#v", reports)
	}
}

func TestValidationUsesTheRequestRenderContextForItsProbe(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_DATA_DIRS", t.TempDir())
	t.Setenv("ASK_PROVIDER_PATH", "")
	directory := filepath.Join(root, "ask", "providers", "sample")
	helperManifest(t, directory, "sample", "ok")
	path := filepath.Join(directory, "provider.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var manifest shared.Manifest
	if err := yaml.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	action := manifest.Actions[ActionValidate]
	action.Argv = append(action.Argv, "{{.Dir}}")
	manifest.Actions[ActionValidate] = action
	raw, err = yaml.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	reports, err := Validate("sample", root)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || !reports[0].OK() {
		t.Fatalf("reports = %#v", reports)
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

func TestMalformedManifestDoesNotHideValidProviders(t *testing.T) {
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
	helperManifest(t, filepath.Join(root, "ask", "providers", "sample"), "sample", "ok")

	known, err := Known()
	if err != nil {
		t.Fatal(err)
	}
	if len(known) != 1 || known[0].Name != "sample" {
		t.Fatalf("Known() = %#v", known)
	}
	if _, found, err := Lookup("sample"); err != nil || !found {
		t.Fatalf("Lookup(sample) found = %v, err = %v", found, err)
	}
	names, err := Names()
	if err != nil || !slices.Equal(names, []string{"sample"}) {
		t.Fatalf("Names() = %#v, err = %v", names, err)
	}
	reports, err := Validate("", root)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 2 || reports[1].Provider != "broken" || reports[1].OK() {
		t.Fatalf("Validate() = %#v", reports)
	}
	if !strings.Contains(reports[1].Checks[0].Target, "provider.yaml") {
		t.Fatalf("malformed report = %#v", reports[1])
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
