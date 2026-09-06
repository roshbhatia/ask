package schema

import "testing"

func TestCheckEnforcesTypeOnlySchema(t *testing.T) {
	shape := Any()
	if err := Check(shape, map[string]any{"answer": "ok"}); err != nil {
		t.Fatalf("valid object: %v", err)
	}
	if err := Check(shape, nil); err == nil {
		t.Fatal("nil structured answer passed an object schema")
	}
}

func TestCheckEnforcesSchemaWithoutLegacyKeywords(t *testing.T) {
	shape := map[string]any{"type": "string", "minLength": 1}
	if err := Check(shape, map[string]any{"answer": "not a string"}); err == nil {
		t.Fatal("object passed a string schema")
	}
}
