// ask-provider-text wraps a one-shot text command in the provider/v1 stream.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	core "github.com/roshbhatia/ask/internal/provider"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "ask-provider-text requires a command")
		os.Exit(2)
	}
	var request core.Envelope
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if request.Version != core.Protocol || request.Action != core.ActionGenerate {
		fmt.Fprintln(os.Stderr, "unsupported provider request")
		os.Exit(2)
	}
	command := exec.CommandContext(context.Background(), os.Args[1], os.Args[2:]...)
	command.Dir = request.Request.Dir
	command.Stdin = bytes.NewReader(request.Request.Input)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	encoder := json.NewEncoder(os.Stdout)
	_ = encoder.Encode(core.Event{Version: core.Protocol, Kind: core.Started, Text: request.Request.Model})
	output, err := command.Output()
	result := &core.Result{Text: strings.TrimSpace(string(output))}
	if err != nil || result.Text == "" {
		result.Failed = true
		result.Reason = strings.TrimSpace(stderr.String())
		if result.Reason == "" && err != nil {
			result.Reason = err.Error()
		}
		if result.Reason == "" {
			result.Reason = "command exited without an answer"
		}
	} else {
		if request.Request.Schema != nil {
			start, end := strings.Index(result.Text, "{"), strings.LastIndex(result.Text, "}")
			if start >= 0 && end >= start {
				_ = json.Unmarshal([]byte(result.Text[start:end+1]), &result.Structured)
			}
			if result.Structured == nil {
				result.Failed = true
				result.Reason = "answered outside the shape --schema asked for"
			}
		}
		_ = encoder.Encode(core.Event{Version: core.Protocol, Kind: core.Text, Text: result.Text})
	}
	if err := encoder.Encode(core.Event{Version: core.Protocol, Kind: core.Done, Result: result}); err != nil {
		os.Exit(1)
	}
}
