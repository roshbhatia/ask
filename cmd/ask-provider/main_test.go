package main

import (
	"strings"
	"testing"

	core "github.com/roshbhatia/ask/internal/provider"
)

func collect() (*[]core.Event, emit) {
	events := []core.Event{}
	return &events, func(event core.Event) error {
		events = append(events, event)
		return nil
	}
}

func TestScanClaudeNormalizesTextAndTools(t *testing.T) {
	events, output := collect()
	stream := strings.Join([]string{
		`{"type":"system","subtype":"init","model":"opus","tools":["Read"]}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"main.go"}},{"type":"text","text":"Checking."}]}}`,
		`{"type":"result","subtype":"success","result":"Done.","structured_output":{"ok":true}}`,
	}, "\n")
	result := scanClaude(strings.NewReader(stream), output)
	if result == nil || result.Failed || result.Text != "Done." || result.Structured["ok"] != true {
		t.Fatalf("got %#v", result)
	}
	if len(*events) != 3 || (*events)[0].Kind != core.Started || (*events)[1].Kind != core.Tool || (*events)[2].Kind != core.Text {
		t.Fatalf("got %#v", *events)
	}
}

func TestScanCodexNormalizesAnswer(t *testing.T) {
	events, output := collect()
	stream := strings.Join([]string{
		`{"type":"thread.started"}`,
		`{"type":"item.started","item":{"id":"1","type":"command_execution","command":"go test ./..."}}`,
		`{"type":"item.completed","item":{"id":"2","type":"agent_message","text":"All tests pass."}}`,
	}, "\n")
	result := scanCodex(strings.NewReader(stream), "gpt-5", output)
	if result == nil || result.Failed || result.Text != "All tests pass." {
		t.Fatalf("got %#v", result)
	}
	if len(*events) != 3 || (*events)[0].Kind != core.Started || (*events)[1].Kind != core.Tool || (*events)[2].Kind != core.Text {
		t.Fatalf("got %#v", *events)
	}
}

func TestStructuredExtractsObjectFromFencedAnswer(t *testing.T) {
	value := structured("answer:\n```json\n{\"ok\":true}\n```")
	if value == nil || value["ok"] != true {
		t.Fatalf("got %#v", value)
	}
}

func TestCodexArgsUseReadOnlyEphemeralSandbox(t *testing.T) {
	args := codexArgs(core.Request{Prompt: "review", Model: "gpt-5"}, "shape.json")
	joined := strings.Join(args, " ")
	for _, required := range []string{"--sandbox read-only", "--ephemeral", "--output-schema shape.json"} {
		if !strings.Contains(joined, required) {
			t.Fatalf("arguments do not contain %q: %q", required, joined)
		}
	}
	if strings.Contains(joined, "dangerously-") {
		t.Fatalf("arguments contain a dangerous bypass: %q", joined)
	}
}
