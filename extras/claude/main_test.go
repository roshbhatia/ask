package main

import (
	"strings"
	"testing"

	core "github.com/roshbhatia/ask/internal/provider"
)

func TestScanNormalizesTextAndTools(t *testing.T) {
	events := []core.Event{}
	stream := `{"type":"system","subtype":"init","model":"opus","tools":["Read"]}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"main.go"}},{"type":"text","text":"Checking."}]}}
{"type":"result","subtype":"success","result":"Done.","structured_output":{"ok":true}}`
	result, err := scan(strings.NewReader(stream), func(event core.Event) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.Failed || result.Text != "Done." || result.Structured["ok"] != true {
		t.Fatalf("got %#v", result)
	}
	if len(events) != 3 || events[0].Kind != core.Started || events[1].Kind != core.Tool || events[2].Kind != core.Text {
		t.Fatalf("got %#v", events)
	}
}

func TestStructuredExtractsObjectFromFencedAnswer(t *testing.T) {
	value := structured(`answer:
~~~json
{"ok":true}
~~~`)
	if value == nil || value["ok"] != true {
		t.Fatalf("got %#v", value)
	}
}
