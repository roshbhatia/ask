package textadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	core "github.com/roshbhatia/ask/internal/provider"
)

func TestCommandProcess(t *testing.T) {
	if os.Getenv("ASK_TEXT_ADAPTER_HELPER") == "" {
		return
	}
	joined := strings.Join(os.Args, " ")
	wantPrompt := "--prompt Review this."
	if !strings.Contains(joined, "--model sample-model") || !strings.Contains(joined, wantPrompt) {
		os.Exit(2)
	}
	if os.Getenv("ASK_TEXT_ADAPTER_EMPTY") == "" {
		for _, want := range []string{"Return one JSON object only", `{"type":"object"}`, "Input:\ndiff"} {
			if !strings.Contains(joined, want) {
				os.Exit(2)
			}
		}
	}
	input, _ := os.ReadFile("/dev/stdin")
	if len(input) != 0 {
		os.Exit(2)
	}
	_, _ = os.Stdout.WriteString(`{"ok":true}`)
	os.Exit(0)
}

func TestWithSchemaContractNamesTheExactShape(t *testing.T) {
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
}

func TestRunMapsDeclarativeCommand(t *testing.T) {
	t.Setenv("ASK_TEXT_ADAPTER_HELPER", "1")
	request := core.Envelope{
		Version: core.Protocol,
		Action:  core.ActionGenerate,
		Request: core.Request{
			Prompt: "Review this.",
			Input:  "diff",
			Model:  "sample-model",
			Schema: map[string]any{"type": "object"},
			Dir:    t.TempDir(),
		},
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	arguments := []string{
		"--input-mode=prompt",
		"--model-flag=--model",
		"--prompt-flag=--prompt",
		"--",
		os.Args[0],
		"-test.run=TestCommandProcess",
		"--",
	}
	var output bytes.Buffer
	if err := Run(arguments, bytes.NewReader(encoded), &output); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&output)
	var started, text, done core.Event
	for _, event := range []*core.Event{&started, &text, &done} {
		if err := decoder.Decode(event); err != nil {
			t.Fatal(err)
		}
	}
	if started.Kind != core.Started || text.Text != `{"ok":true}` || done.Result == nil || done.Result.Structured["ok"] != true {
		t.Fatalf("events = %#v, %#v, %#v", started, text, done)
	}
}

func TestModelNamesAcceptsJSONAndText(t *testing.T) {
	for _, test := range []struct {
		output string
		want   string
	}{
		{output: `{"models":[{"id":"large"},{"id":"small"}]}`, want: "large,small"},
		{output: "large\nsmall\n", want: "large,small"},
		{output: "large\tLarge (High)\nsmall\tSmall (Low)\n", want: "large,small"},
		// cursor-agent: a header line, then "id - Label" rows.
		{output: "Available models\n\nauto - Auto (default)\ngemini-3.8-flash-low - Gemini 3.8 Flash Low\n", want: "auto,gemini-3.8-flash-low"},
		{output: "Fetching models...\nlarge\n", want: "large"},
	} {
		if got := strings.Join(modelNames([]byte(test.output)), ","); got != test.want {
			t.Fatalf("modelNames(%q) = %q, want %q", test.output, got, test.want)
		}
	}
}

func TestRunModelsKeepsAListingFromAFailedCommand(t *testing.T) {
	var listed bytes.Buffer
	if err := runModels(context.Background(), []string{"sh", "-c", "printf 'large\nsmall\n'; exit 1"}, &listed); err != nil {
		t.Fatalf("runModels = %v, want the listing kept", err)
	}
	var payload struct {
		Models []string `json:"models"`
	}
	if err := json.Unmarshal(listed.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(payload.Models, ","); got != "large,small" {
		t.Fatalf("models = %q, want large,small", got)
	}
	if err := runModels(context.Background(), []string{"sh", "-c", "exit 1"}, &bytes.Buffer{}); err == nil {
		t.Fatal("runModels = nil, want the error when the command listed nothing")
	}
}

func TestPromptInputModeAcceptsAnEmptyInput(t *testing.T) {
	t.Setenv("ASK_TEXT_ADAPTER_HELPER", "1")
	t.Setenv("ASK_TEXT_ADAPTER_EMPTY", "1")
	request := core.Envelope{
		Version: core.Protocol,
		Action:  core.ActionGenerate,
		Request: core.Request{
			Prompt: "Review this.",
			Model:  "sample-model",
			Dir:    t.TempDir(),
		},
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	arguments := []string{
		"--input-mode=prompt",
		"--model-flag=--model",
		"--prompt-flag=--prompt",
		"--",
		os.Args[0],
		"-test.run=TestCommandProcess",
		"--",
	}
	var output bytes.Buffer
	if err := Run(arguments, bytes.NewReader(encoded), &output); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&output)
	var started, text, done core.Event
	if err := decoder.Decode(&started); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(&text); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(&done); err != nil {
		t.Fatal(err)
	}
	if started.Kind != core.Started || text.Kind != core.Text || done.Kind != core.Done || done.Result == nil || done.Result.Failed {
		t.Fatalf("events = %#v, %#v, %#v", started, text, done)
	}
}

func TestValidateChecksMappingWithoutRunningCommand(t *testing.T) {
	request, err := json.Marshal(core.Envelope{Version: core.Protocol, Action: core.ActionValidate})
	if err != nil {
		t.Fatal(err)
	}
	arguments := []string{"--validate", "--input-mode=prompt", "--", "not-installed"}
	var output bytes.Buffer
	if err := Run(arguments, bytes.NewReader(request), &output); err != nil {
		t.Fatal(err)
	}
	var response core.ValidationResponse
	if err := json.NewDecoder(&output).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Version != core.Protocol || response.Status != "ok" {
		t.Fatalf("response = %#v", response)
	}
}
