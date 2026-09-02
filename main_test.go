package main

import (
	"strings"
	"testing"

	"github.com/roshbhatia/ask/internal/templates"
)

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
		Prompt: "Review {{repo}} for {{focus}}.",
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
		Prompt:    "Review {{repo}}.",
		Variables: []templates.Variable{{Name: "repo", Required: true}},
	}); err != nil {
		t.Fatal(err)
	}

	_, _, err := promptFromTemplate(options{template: "review", quiet: true})
	if err == nil || !strings.Contains(err.Error(), "needs --var for: repo") {
		t.Fatalf("got %v", err)
	}
}
