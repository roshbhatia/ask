package templates

import (
	"strings"
	"testing"
)

func TestResolveUsesDefaultsAndSuppliedValues(t *testing.T) {
	prompt := Prompt{
		Name:   "review",
		Prompt: "Review {{ repo }} for {{focus}}.",
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
		Prompt:    "Review {{repo}}.",
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

func TestSaveAndLoadTemplates(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	shape := map[string]any{"type": "object"}
	if _, err := SaveSchema(Schema{Name: "result", Schema: shape}); err != nil {
		t.Fatal(err)
	}
	if _, err := SavePrompt(Prompt{Name: "review", Prompt: "Review it.", Schema: "result"}); err != nil {
		t.Fatal(err)
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
