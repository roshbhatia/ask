package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/roshbhatia/go-utils/completion"
	providerlib "github.com/roshbhatia/go-utils/provider"
	"go.yaml.in/yaml/v3"

	"github.com/roshbhatia/ask/internal/provider"
	"github.com/roshbhatia/ask/internal/schema"
	"github.com/roshbhatia/ask/internal/store"
	"github.com/roshbhatia/ask/internal/templates"
)

type fixedProvider struct {
	result provider.Result
	runs   int
}

func (p *fixedProvider) Name() string { return "fixed" }

func (p *fixedProvider) Run(context.Context, provider.Request) (<-chan provider.Event, error) {
	p.runs++
	events := make(chan provider.Event, 1)
	events <- provider.Event{Version: provider.Protocol, Kind: provider.Done, Result: &p.result}
	close(events)
	return events, nil
}

func TestJSONRunRejectsPlainTextResult(t *testing.T) {
	agent := &fixedProvider{result: provider.Result{Text: "plain text"}}
	_, err := converse(provider.Request{}, schema.Any(), options{quiet: true, timeout: time.Second}, agent)
	if err == nil || !strings.Contains(err.Error(), "outside the shape") {
		t.Fatalf("error = %v, want structured-answer failure", err)
	}
	if agent.runs != rounds {
		t.Fatalf("provider ran %d times, want %d repair rounds", agent.runs, rounds)
	}
}

func TestProviderValidateJSONExitsNonzeroForFailedCheck(t *testing.T) {
	configHome := t.TempDir()
	providers := filepath.Join(configHome, "ask", "providers", "broken")
	if err := os.MkdirAll(providers, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(providers, "provider.yaml"), []byte("not: a: manifest\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=TestAskHelperProcess", "--", "provider", "validate", "--json")
	command.Env = append(os.Environ(),
		"ASK_TEST_HELPER=1",
		"ASK_PROVIDER_PATH=",
		"XDG_CONFIG_HOME="+configHome,
		"XDG_DATA_HOME="+t.TempDir(),
		"XDG_DATA_DIRS="+t.TempDir(),
	)
	var stdout bytes.Buffer
	command.Stdout = &stdout
	err := command.Run()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("error = %v, want exit 1", err)
	}
	var reports []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &reports); err != nil {
		t.Fatalf("stdout = %q: %v", stdout.String(), err)
	}
	if len(reports) != 1 || reports[0]["provider"] != "broken" {
		t.Fatalf("reports = %#v", reports)
	}
}

func TestAskHelperProcess(t *testing.T) {
	if os.Getenv("ASK_TEST_HELPER") != "1" {
		return
	}
	separator := slices.Index(os.Args, "--")
	if separator < 0 {
		os.Exit(2)
	}
	os.Args = append([]string{"ask"}, os.Args[separator+1:]...)
	main()
	os.Exit(0)
}

func TestPromptFromTemplateRendersAndAssociatesSchema(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := templates.SaveSchema(templates.Schema{
		Name: "review-result",
		Schema: map[string]any{
			"type": "object",
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := templates.SavePrompt(templates.Prompt{
		Name:   "review",
		Prompt: "Review {{.repo}} for {{.focus}}.",
		Schema: "review-result",
		Variables: []templates.Variable{
			{Name: "repo", Required: true},
			{Name: "focus", Default: "correctness"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	prompt, pinned, err := promptFromTemplate(options{
		template: "review",
		vars:     []string{"repo=ask"},
		quiet:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if prompt != "Review ask for correctness." {
		t.Fatalf("got prompt %q", prompt)
	}
	if pinned.Schema != "review-result" {
		t.Fatalf("got schema %q", pinned.Schema)
	}
}

func TestPromptFromTemplateNamesMissingVariables(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := templates.SavePrompt(templates.Prompt{
		Name:      "review",
		Prompt:    "Review {{.repo}}.",
		Variables: []templates.Variable{{Name: "repo", Required: true}},
	}); err != nil {
		t.Fatal(err)
	}

	_, _, err := promptFromTemplate(options{template: "review", quiet: true})
	if err == nil || !strings.Contains(err.Error(), "needs --var for: repo") {
		t.Fatalf("got %v", err)
	}
}

func TestPromptFromTemplateRejectsInvalidTypedValue(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := templates.SavePrompt(templates.Prompt{
		Name:      "review",
		Prompt:    "{{if .strict}}Review it.{{end}}",
		Variables: []templates.Variable{{Name: "strict", Type: "bool", Required: true}},
	}); err != nil {
		t.Fatal(err)
	}

	_, _, err := promptFromTemplate(options{template: "review", vars: []string{"strict=perhaps"}, quiet: true})
	if err == nil || !strings.Contains(err.Error(), `variable "strict": want bool`) {
		t.Fatalf("got %v", err)
	}
}

func TestPromptSaveUsesLastPromptAndAssociatedSchema(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if _, err := templates.SaveSchema(templates.Schema{
		Name:   "review-result",
		Schema: map[string]any{"type": "object"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRun(nil, "Review {{.repo}}. {{if .strict}}Require migration tests.{{end}}"); err != nil {
		t.Fatal(err)
	}

	cmd := promptCommand()
	cmd.SetArgs([]string{
		"save", "code-review",
		"--schema", "review-result",
		"--provider", "local-model",
		"--model", "light",
		"--variable", "repo:string",
		"--variable", "strict:bool=true",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	prompt, err := templates.LoadPrompt("code-review")
	if err != nil {
		t.Fatal(err)
	}
	if prompt.Schema != "review-result" || len(prompt.Variables) != 2 {
		t.Fatalf("saved prompt = %#v", prompt)
	}
	if prompt.Provider != "local-model" || prompt.Model != "light" {
		t.Fatalf("saved pins = %q, %q", prompt.Provider, prompt.Model)
	}
	rendered, pinned, err := promptFromTemplate(options{
		template: "code-review",
		vars:     []string{"repo=payments"},
		quiet:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if pinned.Schema != "review-result" || rendered != "Review payments. Require migration tests." {
		t.Fatalf("rendered = %q, schema = %q", rendered, pinned.Schema)
	}
	if pinned.Provider != "local-model" || pinned.Model != "light" {
		t.Fatalf("loaded pins = %q, %q", pinned.Provider, pinned.Model)
	}
}

func TestWithPinsLetsFlagsWin(t *testing.T) {
	pinned := templates.Prompt{Schema: "shape", Provider: "pinned", Model: "light"}
	folded := withPins(options{template: "t"}, pinned)
	if folded.schemaTemplate != "shape" || folded.provider != "pinned" || folded.model != "light" {
		t.Fatalf("bare flags took nothing from the template: %#v", folded)
	}
	if got := requestedModel(folded); got != "light" {
		t.Fatalf("requestedModel = %q, want the template's light pin", got)
	}

	folded = withPins(options{template: "t", provider: "flag", model: "id", spec: "a:string"}, pinned)
	if folded.provider != "flag" || folded.model != "id" || folded.schemaTemplate != "" {
		t.Fatalf("flags lost to the template: %#v", folded)
	}

	folded = withPins(options{template: "t", light: true}, templates.Prompt{Model: "heavy"})
	if folded.model != "" || requestedModel(folded) != "light" {
		t.Fatalf("-L lost to the template's model pin: %#v", folded)
	}
	if got := requestedModel(options{model: "id"}); got != "id" {
		t.Fatalf("requestedModel = %q, want the literal id", got)
	}
}

func TestCompletionSpecKeepsNestedDynamicCompleters(t *testing.T) {
	spec := completionSpec(command(new(options)))
	wants := []struct {
		path []string
		flag string
		kind string
	}{
		{flag: "provider", kind: "providers"},
		{flag: "model", kind: "models"},
		{flag: "schema", kind: "schemas"},
		{flag: "template", kind: "prompt-templates"},
		{flag: "schema-template", kind: "schema-templates"},
		{path: []string{"provider", "validate"}, kind: "providers"},
		{path: []string{"prompt", "show"}, kind: "prompt-templates"},
		{path: []string{"schema", "show"}, kind: "schema-templates"},
		{path: []string{"prompt", "save"}, flag: "schema", kind: "schema-templates"},
		{path: []string{"prompt", "save"}, flag: "provider", kind: "providers"},
		{path: []string{"prompt", "save"}, flag: "model", kind: "models"},
	}
	for _, want := range wants {
		command := findCompletionCommand(t, spec, want.path...)
		invocation := command.CompletionCommand
		if want.flag != "" {
			invocation = findCompletionFlag(t, command, want.flag).CompletionCommand
		}
		expected := []string{spec.Name, "__values", want.kind}
		if want.kind == "models" || want.kind == "schemas" {
			expected = append(expected, completion.ContextPlaceholder)
		}
		if !slices.Equal(invocation, expected) {
			t.Fatalf("%v --%s completion = %#v, want %#v", want.path, want.flag, invocation, expected)
		}
	}

	for _, shell := range []string{"bash", "zsh", "fish", "nu"} {
		generated, err := completion.Generate(shell, spec)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"providers", "models", "schemas", "prompt-templates", "schema-templates"} {
			if !strings.Contains(generated, want) {
				t.Fatalf("%s completion lacks %q", shell, want)
			}
		}
	}
}

func TestSchemaCompletionValueReadsFlagContext(t *testing.T) {
	tests := map[string]string{
		"ask --schema ":                      "",
		"ask --schema files:":                "files:",
		"ask -s @schema/re":                  "@schema/re",
		"ask --schema=summary:string":        "summary:string",
		"ask -s=summary:string":              "summary:string",
		"ask --schema 'name:string, count:'": "name:string, count:",
		"summary:string":                     "summary:string",
	}
	for context, want := range tests {
		if got := schemaCompletionValue(context); got != want {
			t.Errorf("schemaCompletionValue(%q) = %q, want %q", context, got, want)
		}
	}
}

func TestSchemaCompletionValuesAreShellSafe(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=TestAskHelperProcess", "--", "__values", "schemas", "ask --schema ")
	command.Env = append(os.Environ(), "ASK_TEST_HELPER=1")
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(output, []byte{'\t'}) {
		t.Fatalf("schema completion contains a tab-separated description: %q", output)
	}
	if !strings.Contains(string(output), "ok:bool, reason:string\n") {
		t.Fatalf("schema completion lacks the expected candidate: %q", output)
	}
}

func TestCompletionOptionsReadsProviderFromShellContext(t *testing.T) {
	for _, context := range []string{
		"ask --provider local-model --model ",
		"ask -p local-model --model ",
		"ask --provider=local-model --model ",
		"ask -p=local-model --model ",
		`ask -p 'local-model' --model `,
		"ask -p first --provider local-model --model ",
	} {
		if got := completionOptions(context).provider; got != "local-model" {
			t.Fatalf("completionOptions(%q).provider = %q", context, got)
		}
	}
}

func findCompletionCommand(t *testing.T, command completion.Command, path ...string) completion.Command {
	t.Helper()
	for _, name := range path {
		found := false
		for _, child := range command.Subcommands {
			if child.Name == name {
				command = child
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("completion command %q not found", strings.Join(path, " "))
		}
	}
	return command
}

func findCompletionFlag(t *testing.T, command completion.Command, name string) completion.Flag {
	t.Helper()
	for _, flag := range command.Flags {
		if flag.Name == name {
			return flag
		}
	}
	t.Fatalf("completion flag --%s not found", name)
	return completion.Flag{}
}

// captureProvider installs a provider whose adapter is this test binary. The
// adapter writes the envelope it receives to the returned path, so a test can
// see exactly what ask handed it.
func captureProvider(t *testing.T, configHome, name string, defaults providerlib.Defaults) string {
	t.Helper()
	capture := filepath.Join(t.TempDir(), name+".json")
	env := map[string]string{"ASK_TEST_ADAPTER": "1", "ASK_TEST_CAPTURE": capture}
	manifest := providerlib.Manifest{
		Version:     provider.Protocol,
		Name:        name,
		Description: name + " test provider",
		Command:     []string{os.Args[0], "-test.run=TestAskAdapterProcess", "--"},
		Actions: map[string]providerlib.Action{
			provider.ActionGenerate: {Description: "generate", Env: env},
			provider.ActionValidate: {Description: "validate", Env: env},
		},
		Defaults: defaults,
	}
	raw, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(configHome, "ask", "providers", name)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "provider.yaml"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return capture
}

func TestAskAdapterProcess(t *testing.T) {
	if os.Getenv("ASK_TEST_ADAPTER") != "1" {
		return
	}
	var envelope provider.Envelope
	if err := json.NewDecoder(os.Stdin).Decode(&envelope); err != nil {
		os.Exit(2)
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		os.Exit(2)
	}
	if err := os.WriteFile(os.Getenv("ASK_TEST_CAPTURE"), raw, 0o600); err != nil {
		os.Exit(2)
	}
	encoder := json.NewEncoder(os.Stdout)
	if envelope.Action == provider.ActionValidate {
		_ = encoder.Encode(provider.ValidationResponse{Version: provider.Protocol, Status: "ok"})
		os.Exit(0)
	}
	_ = encoder.Encode(provider.Event{Version: provider.Protocol, Kind: provider.Done, Result: &provider.Result{Text: "answer"}})
	os.Exit(0)
}

func capturedRequest(t *testing.T, capture string) provider.Request {
	t.Helper()
	raw, err := os.ReadFile(capture)
	if err != nil {
		t.Fatalf("the adapter captured nothing: %v", err)
	}
	var envelope provider.Envelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(capture); err != nil {
		t.Fatal(err)
	}
	return envelope.Request
}

func TestLightFlagAndTemplatePinsReachTheProviderResolved(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_DATA_DIRS", t.TempDir())
	t.Setenv("ASK_PROVIDER_PATH", "")
	t.Setenv("ASK_PROVIDER", "")
	tiered := captureProvider(t, configHome, "tiered", providerlib.Defaults{Model: "large", Light: "small"})
	heavy := captureProvider(t, configHome, "heavy", providerlib.Defaults{Model: "large"})
	for _, prompt := range []templates.Prompt{
		{Name: "triage", Prompt: "Triage {{.what}}.", Provider: "tiered", Model: "light", Variables: []templates.Variable{{Name: "what", Default: "it"}}},
		{Name: "deep", Prompt: "Study it.", Provider: "tiered", Model: "large-2026"},
	} {
		if _, err := templates.SavePrompt(prompt); err != nil {
			t.Fatal(err)
		}
	}
	base := options{quiet: true, timeout: 30 * time.Second}

	cases := []struct {
		name    string
		opts    options
		capture string
		model   string
	}{
		{name: "-L resolves to the light id", opts: options{prompt: "hi", provider: "tiered", light: true}, capture: tiered, model: "small"},
		{name: "-m passes a literal through", opts: options{prompt: "hi", provider: "tiered", model: "custom"}, capture: tiered, model: "custom"},
		{name: "no model means the default", opts: options{prompt: "hi", provider: "tiered"}, capture: tiered, model: "large"},
		{name: "template pins provider and light", opts: options{template: "triage"}, capture: tiered, model: "small"},
		{name: "-m beats the template model", opts: options{template: "triage", model: "custom"}, capture: tiered, model: "custom"},
		{name: "-L beats the template model", opts: options{template: "deep", light: true}, capture: tiered, model: "small"},
		{name: "template literal id passes through", opts: options{template: "deep"}, capture: tiered, model: "large-2026"},
		{name: "-p beats the template provider", opts: options{template: "deep", provider: "heavy"}, capture: heavy, model: "large-2026"},
	}
	for _, tc := range cases {
		opts := tc.opts
		opts.quiet, opts.timeout = base.quiet, base.timeout
		if err := run(opts); err != nil {
			t.Fatalf("%s: run: %v", tc.name, err)
		}
		request := capturedRequest(t, tc.capture)
		if request.Model != tc.model {
			t.Fatalf("%s: provider received model %q, want %q", tc.name, request.Model, tc.model)
		}
		if opts.prompt == "" && !strings.HasSuffix(request.Prompt, "it.") {
			t.Fatalf("%s: provider received prompt %q, want the rendered template", tc.name, request.Prompt)
		}
	}
}

func TestLightFlagFailsOnAProviderWithoutALightModel(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_DATA_DIRS", t.TempDir())
	t.Setenv("ASK_PROVIDER_PATH", "")
	t.Setenv("ASK_PROVIDER", "")
	heavy := captureProvider(t, configHome, "heavy", providerlib.Defaults{Model: "large"})
	if _, err := templates.SavePrompt(templates.Prompt{Name: "triage", Prompt: "Triage it.", Model: "light"}); err != nil {
		t.Fatal(err)
	}
	base := options{quiet: true, timeout: 30 * time.Second}

	for name, opts := range map[string]options{
		"-L":              {prompt: "hi", provider: "heavy", light: true},
		"template light":  {template: "triage", provider: "heavy"},
		"-m light":        {prompt: "hi", provider: "heavy", model: "light"},
		"default is fine": {prompt: "hi", provider: "heavy", model: "default"},
	} {
		opts.quiet, opts.timeout = base.quiet, base.timeout
		err := run(opts)
		if name == "default is fine" {
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if got := capturedRequest(t, heavy).Model; got != "large" {
				t.Fatalf("%s: provider received %q, want the default", name, got)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), "no light model") {
			t.Fatalf("%s: error = %v, want a missing light model", name, err)
		}
		if _, statErr := os.Stat(heavy); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("%s: the provider ran anyway", name)
		}
	}

	err := run(options{prompt: "hi", provider: "heavy", model: "large", light: true, quiet: true, timeout: time.Second})
	if err == nil || !strings.Contains(err.Error(), "either --model or --light") {
		t.Fatalf("-m with -L: error = %v", err)
	}
}

func TestModelCompletionOffersDeclaredRolesFirst(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_DATA_DIRS", t.TempDir())
	t.Setenv("ASK_PROVIDER_PATH", "")
	t.Setenv("ASK_PROVIDER", "")
	captureProvider(t, configHome, "tiered", providerlib.Defaults{Model: "large", Light: "small"})
	captureProvider(t, configHome, "heavy", providerlib.Defaults{Model: "large"})
	captureProvider(t, configHome, "bare", providerlib.Defaults{})

	if got := modelNames(options{provider: "tiered"}); !slices.Equal(got, []string{"light", "default"}) {
		t.Fatalf("tiered models = %#v", got)
	}
	if got := modelNames(options{provider: "heavy"}); !slices.Equal(got, []string{"default"}) {
		t.Fatalf("heavy models = %#v", got)
	}
	if got := modelNames(options{provider: "bare"}); len(got) != 0 {
		t.Fatalf("bare models = %#v", got)
	}
	described := models(options{provider: "tiered"})
	if len(described) != 2 || !strings.HasPrefix(described[0], "light\t") {
		t.Fatalf("described models = %#v", described)
	}
}
