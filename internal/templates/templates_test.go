package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveUsesDefaultsAndSuppliedValues(t *testing.T) {
	prompt := Prompt{
		Name:   "review",
		Prompt: "Review {{.repo}} for {{.focus}}.",
		Variables: []Variable{
			{Name: "repo", Required: true},
			{Name: "focus", Default: "correctness"},
		},
	}

	got, missing, err := Resolve(prompt, map[string]string{"repo": "ask"})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("unexpected missing variables: %#v", missing)
	}
	if got != "Review ask for correctness." {
		t.Fatalf("got %q", got)
	}
}

func TestResolveReportsMissingVariables(t *testing.T) {
	prompt := Prompt{
		Name:      "review",
		Prompt:    "Review {{.repo}}.",
		Variables: []Variable{{Name: "repo", Required: true}},
	}

	_, missing, err := Resolve(prompt, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 1 || missing[0].Name != "repo" {
		t.Fatalf("got %#v", missing)
	}
}

func TestResolveRejectsUnknownVariables(t *testing.T) {
	prompt := Prompt{Name: "review", Prompt: "Review it."}
	_, _, err := Resolve(prompt, map[string]string{"repo": "ask"})
	if err == nil || !strings.Contains(err.Error(), "no variable") {
		t.Fatalf("got %v", err)
	}
}

func TestResolveValidatesTypedVariables(t *testing.T) {
	prompt := Prompt{
		Name: "release",
		Prompt: `{{if .strict}}Review {{.repo}} attempt {{.attempt}} at {{.threshold}}.
Labels: {{json .labels}}{{end}}`,
		Variables: []Variable{
			{Name: "repo", Type: typeString, Required: true},
			{Name: "strict", Type: typeBool, Default: "true"},
			{Name: "attempt", Type: typeInt, Default: "2"},
			{Name: "threshold", Type: typeNumber, Default: "0.75"},
			{Name: "labels", Type: typeJSON, Default: `["correctness","migration"]`},
		},
	}

	got, missing, err := Resolve(prompt, map[string]string{"repo": "payments"})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("unexpected missing variables: %#v", missing)
	}
	want := "Review payments attempt 2 at 0.75.\nLabels: [\"correctness\",\"migration\"]"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	_, _, err = Resolve(prompt, map[string]string{"repo": "payments", "strict": "sometimes"})
	if err == nil || !strings.Contains(err.Error(), `variable "strict": want bool`) {
		t.Fatalf("got %v", err)
	}
}

func TestSavePromptRejectsUndeclaredVariableInSkippedBranch(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, body := range []string{
		"{{if .strict}}{{.undeclared}}{{end}}",
		"{{if .strict}}{{with .}}{{.undeclared}}{{end}}{{end}}",
		"{{$root := .}}{{if .strict}}{{$root.undeclared}}{{end}}",
		`{{$root := .}}{{if .strict}}{{index $root "undeclared"}}{{end}}`,
		`{{define "body"}}{{.undeclared}}{{end}}{{template "body" .}}`,
		`{{index . "undeclared"}}`,
	} {
		_, err := SavePrompt(Prompt{
			Name:   "review",
			Prompt: body,
			Variables: []Variable{
				{Name: "strict", Type: typeBool, Default: "true"},
			},
		})
		if err == nil || !strings.Contains(err.Error(), `uses undeclared variable "undeclared"`) {
			t.Fatalf("prompt %q: got %v", body, err)
		}
	}
}

func TestSavePromptAllowsFieldsWithinJSONVariable(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_, err := SavePrompt(Prompt{
		Name:   "review",
		Prompt: "{{with .payload}}{{.name}}{{end}}",
		Variables: []Variable{
			{Name: "payload", Type: typeJSON, Default: `{"name":"payments"}`},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestResolvePreservesJSONIntegerPrecision(t *testing.T) {
	prompt := Prompt{
		Name:      "inspect",
		Prompt:    "{{json .payload}}",
		Variables: []Variable{{Name: "payload", Type: typeJSON, Required: true}},
	}

	got, missing, err := Resolve(prompt, map[string]string{"payload": `{"id":9007199254740993}`})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("unexpected missing variables: %#v", missing)
	}
	if got != `{"id":9007199254740993}` {
		t.Fatalf("got %q", got)
	}
}

func TestParseVariablesAcceptsTypedDeclarations(t *testing.T) {
	variables, err := ParseVariables([]string{"repo:string", "strict:bool=true", "attempt:int=2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(variables) != 3 {
		t.Fatalf("got %#v", variables)
	}
	if variables[0].Type != typeString || !variables[0].Required {
		t.Fatalf("repo = %#v", variables[0])
	}
	if variables[1].Type != typeBool || variables[1].Default != "true" || variables[1].Required {
		t.Fatalf("strict = %#v", variables[1])
	}

	if _, err := ParseVariables([]string{"strict:bool=perhaps"}); err == nil {
		t.Fatal("ParseVariables accepted an invalid bool default")
	}
	if _, err := ParseVariables([]string{"repo:duration"}); err == nil {
		t.Fatal("ParseVariables accepted an unknown type")
	}
}

func TestSaveAndLoadTemplates(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	shape := map[string]any{"type": "object"}
	if _, err := SaveSchema(Schema{Name: "result", Schema: shape}); err != nil {
		t.Fatal(err)
	}
	if _, err := SavePrompt(Prompt{Name: "review", Prompt: "Review it.", Schema: "result"}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(PromptDir(), "review.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), "# yaml-language-server: $schema="+promptSchemaURL+"\n") {
		t.Fatalf("saved prompt lacks schema directive: %q", raw)
	}

	prompt, err := LoadPrompt("review")
	if err != nil {
		t.Fatal(err)
	}
	if prompt.Schema != "result" {
		t.Fatalf("got schema %q", prompt.Schema)
	}
	schema, err := LoadSchema("result")
	if err != nil {
		t.Fatal(err)
	}
	if schema.Schema["type"] != "object" {
		t.Fatalf("got %#v", schema.Schema)
	}
}

func TestLoadPromptRejectsUnknownFieldsAndSupportsVersionOne(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := os.MkdirAll(PromptDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(PromptDir(), "review.yaml")
	unknown := []byte("version: ask.prompt/v1\nname: review\nprompt: Review it.\nunknown: true\n")
	if err := os.WriteFile(path, unknown, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPrompt("review"); err == nil || !strings.Contains(err.Error(), "field unknown not found") {
		t.Fatalf("got %v", err)
	}

	legacy := []byte("version: ask.prompt/v1\nname: review\nprompt: 'Review {{repo-name}}.'\nvariables:\n  - name: repo-name\n    required: true\n")
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	prompt, err := LoadPrompt("review")
	if err != nil {
		t.Fatal(err)
	}
	got, missing, err := Resolve(prompt, map[string]string{"repo-name": "payments"})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 || got != "Review payments." {
		t.Fatalf("got %q with missing %#v", got, missing)
	}

	literal := []byte("version: ask.prompt/v1\nname: review\nprompt: 'Explain {{.Enabled}}'\n")
	if err := os.WriteFile(path, literal, 0o600); err != nil {
		t.Fatal(err)
	}
	prompt, err = LoadPrompt("review")
	if err != nil {
		t.Fatal(err)
	}
	got, missing, err = Resolve(prompt, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 || got != "Explain {{.Enabled}}" {
		t.Fatalf("got %q with missing %#v", got, missing)
	}
}

func TestGeneratedTemplateSchemasDescribeTypes(t *testing.T) {
	promptSchema, err := PromptSchema()
	if err != nil {
		t.Fatal(err)
	}
	schemaSchema, err := SchemaTemplateSchema()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"ask.prompt/v2", "variables", "bool", "number", "json"} {
		if !strings.Contains(string(promptSchema), want) {
			t.Fatalf("prompt schema does not contain %q", want)
		}
	}
	for _, want := range []string{"ask.schema/v1", "schema"} {
		if !strings.Contains(string(schemaSchema), want) {
			t.Fatalf("schema template schema does not contain %q", want)
		}
	}
}
