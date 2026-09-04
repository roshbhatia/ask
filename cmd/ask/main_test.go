package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/roshbhatia/go-utils/completion"

	"github.com/roshbhatia/ask/internal/store"
	"github.com/roshbhatia/ask/internal/templates"
)

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

	prompt, schemaName, err := promptFromTemplate(options{
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
	if schemaName != "review-result" {
		t.Fatalf("got schema %q", schemaName)
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
	rendered, schemaName, err := promptFromTemplate(options{
		template: "code-review",
		vars:     []string{"repo=payments"},
		quiet:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if schemaName != "review-result" || rendered != "Review payments. Require migration tests." {
		t.Fatalf("rendered = %q, schema = %q", rendered, schemaName)
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
		{flag: "template", kind: "prompt-templates"},
		{flag: "schema-template", kind: "schema-templates"},
		{path: []string{"provider", "validate"}, kind: "providers"},
		{path: []string{"prompt", "show"}, kind: "prompt-templates"},
		{path: []string{"schema", "show"}, kind: "schema-templates"},
		{path: []string{"prompt", "save"}, flag: "schema", kind: "schema-templates"},
	}
	for _, want := range wants {
		command := findCompletionCommand(t, spec, want.path...)
		invocation := command.CompletionCommand
		if want.flag != "" {
			invocation = findCompletionFlag(t, command, want.flag).CompletionCommand
		}
		expected := []string{spec.Name, "__values", want.kind}
		if want.kind == "models" {
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
		for _, want := range []string{"providers", "models", "prompt-templates", "schema-templates"} {
			if !strings.Contains(generated, want) {
				t.Fatalf("%s completion lacks %q", shell, want)
			}
		}
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
