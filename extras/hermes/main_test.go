package main

import (
	"strings"
	"testing"
)

func TestWithSchemaContractAndStructuredResult(t *testing.T) {
	prompt, err := withSchemaContract("Answer this.", map[string]any{
		"type":     "object",
		"required": []string{"answer"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Answer this.", "Return one JSON object only.", `"required":["answer"]`} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("schema prompt lacks %q: %s", want, prompt)
		}
	}
	result := structured("result:\n```json\n{\"answer\":\"yes\"}\n```")
	if result["answer"] != "yes" {
		t.Fatalf("structured result = %#v", result)
	}
}
