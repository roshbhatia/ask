package evaluation

import (
	"encoding/json"
	"testing"
)

func TestState(t *testing.T) {
	for _, tc := range []struct {
		input, format string
		valid         bool
	}{
		{"hello\nworld", "auto", true}, {"{broken", "auto", false}, {"{broken", "text", true},
		{"", "text", false}, {"\x00abc", "text", false}, {"\xff", "auto", false},
		{"[]", "json", true}, {"null", "json", true}, {"true", "json", true},
		{"{} {}", "json", false}, {"hi", "invalid", false},
		{`{"id":1,"id":2}`, "json", false},
	} {
		t.Run(tc.input+tc.format, func(t *testing.T) {
			_, err := State([]byte(tc.input), tc.format)
			if (err == nil) != tc.valid {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestPreserveLargeJSONInteger(t *testing.T) {
	raw := []byte(`{"id":9007199254740993}`)
	state, err := State(raw, "json")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != string(raw) {
		t.Fatalf("changed JSON: %s", encoded)
	}
}

func TestRejectDuplicateQuestionIDs(t *testing.T) {
	var questions Questions
	err := Decode([]byte(`{"a":{"type":"boolean","instructions":"First"},"a":{"type":"boolean","instructions":"Second"}}`), &questions)
	if err == nil {
		t.Fatal("accepted duplicate question IDs")
	}
}

func TestAnswerContract(t *testing.T) {
	q := Questions{"b": {Type: "boolean", Instructions: "Does it pass?"}, "c": {Type: "choice", Instructions: "Which?", Choices: map[string]string{"a": "A", "b": "B"}}, "s": {Type: "score", Instructions: "How much?", Levels: []string{"low", "high"}}}
	if err := q.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, raw     string
		native, valid bool
	}{
		{"false is valid", `{"b":{"value":false},"c":{"value":"a"},"s":{"value":0.5}}`, false, true},
		{"missing", `{"b":{"value":true}}`, false, false},
		{"extra", `{"b":{"value":true},"c":{"value":"a"},"s":{"value":0},"x":{"value":true}}`, false, false},
		{"bad enum", `{"b":{"value":true},"c":{"value":"unknown"},"s":{"value":0}}`, false, false},
		{"score range", `{"b":{"value":true},"c":{"value":"a"},"s":{"value":2}}`, false, false},
		{"string boolean", `{"b":{"value":"false"},"c":{"value":"a"},"s":{"value":0}}`, false, false},
		{"native", `{"b":{"value":false,"probability":0.01},"c":{"value":"a","distribution":{"a":0.8,"b":0.2}},"s":{"value":0.5}}`, true, true},
		{"fake probability", `{"b":{"value":true,"probability":0.9},"c":{"value":"a"},"s":{"value":0}}`, false, false},
		{"contradiction", `{"b":{"value":false,"probability":0.9},"c":{"value":"a"},"s":{"value":0}}`, true, false},
		{"unnormalized", `{"b":{"value":true},"c":{"value":"a","distribution":{"a":0.8,"b":0.8}},"s":{"value":0}}`, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var answers map[string]Answer
			if err := json.Unmarshal([]byte(tc.raw), &answers); err != nil {
				t.Fatal(err)
			}
			err := q.Check(answers, tc.native)
			if (err == nil) != tc.valid {
				t.Fatalf("error = %v", err)
			}
		})
	}
}
