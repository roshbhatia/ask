package main

import (
	"strings"
	"testing"

	core "github.com/roshbhatia/ask/internal/provider"
)

func TestScanNormalizesAnswer(t *testing.T) {
	events := []core.Event{}
	stream := `{"type":"thread.started"}
{"type":"item.started","item":{"id":"1","type":"command_execution","command":"go test ./..."}}
{"type":"item.completed","item":{"id":"2","type":"agent_message","text":"All tests pass."}}`
	result, err := scan(strings.NewReader(stream), "gpt-5", func(event core.Event) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.Failed || result.Text != "All tests pass." {
		t.Fatalf("got %#v", result)
	}
	if len(events) != 3 || events[0].Kind != core.Started || events[1].Kind != core.Tool || events[2].Kind != core.Text {
		t.Fatalf("got %#v", events)
	}
}

func TestArgumentsUseReadOnlyEphemeralSandbox(t *testing.T) {
	args := arguments(core.Request{Prompt: "review", Model: "gpt-5"}, "shape.json")
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
