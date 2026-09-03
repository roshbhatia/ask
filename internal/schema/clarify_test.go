package schema

import "testing"

func TestClarificationRule(t *testing.T) {
	want := `If this request is ambiguous and a wrong guess costs more than a question does, answer with {"clarify": "<one short question>"} and nothing else. Ask at most one question at a time. Otherwise answer in the shape asked for.`
	if Rule != want {
		t.Fatalf("got %q, want %q", Rule, want)
	}
}
