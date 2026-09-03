// ask-provider-claude adapts Claude Code's JSON stream to provider/v1.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/roshbhatia/ask/internal/process"
	core "github.com/roshbhatia/ask/internal/provider"
)

const offShape = "answered outside the shape --schema asked for"

type emit func(core.Event) error

type line struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	Message struct {
		Content []struct {
			Type string          `json:"type"`
			Text string          `json:"text"`
			Name string          `json:"name"`
			Args json.RawMessage `json:"input"`
		} `json:"content"`
	} `json:"message"`
	Model            string         `json:"model"`
	Tools            []string       `json:"tools"`
	Result           string         `json:"result"`
	StructuredOutput map[string]any `json:"structured_output"`
	IsError          bool           `json:"is_error"`
}

func main() {
	var request core.Envelope
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		fail(err)
	}
	if request.Version != core.Protocol {
		fail(errors.New("unsupported provider protocol"))
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if request.Action == core.ActionValidate {
		if err := json.NewEncoder(os.Stdout).Encode(core.ValidationResponse{Version: core.Protocol, Status: "ok"}); err != nil {
			fail(err)
		}
		return
	}
	if request.Action == core.ActionModels {
		if err := json.NewEncoder(os.Stdout).Encode(map[string]any{"version": core.Protocol, "models": models(ctx)}); err != nil {
			fail(err)
		}
		return
	}
	if request.Action != core.ActionGenerate {
		fail(errors.New("unsupported provider action"))
	}
	encoder := json.NewEncoder(os.Stdout)
	output := func(event core.Event) error {
		event.Version = core.Protocol
		return encoder.Encode(event)
	}
	if err := generate(ctx, request.Request, output); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(2)
}

func generate(ctx context.Context, request core.Request, output emit) error {
	args := []string{"--print", "--output-format", "stream-json", "--verbose", "--no-session-persistence"}
	if request.Model != "" {
		args = append(args, "--model", request.Model)
	}
	if request.Schema != nil {
		encoded, err := json.Marshal(request.Schema)
		if err != nil {
			return err
		}
		args = append(args, "--json-schema", string(encoded))
	}
	args = append(args, "--", request.Prompt)
	command := process.CommandContext(ctx, "claude", args...)
	command.Dir, command.Stdin = request.Dir, strings.NewReader(request.Input)
	return runCommand(command, func(reader io.Reader, output emit) (*core.Result, error) {
		result, err := scan(reader, output)
		if err != nil {
			return nil, err
		}
		if result != nil && request.Schema != nil && !result.Failed && result.Structured == nil {
			result.Structured = structured(result.Text)
			if result.Structured == nil {
				result.Failed, result.Reason = true, offShape
			}
		}
		return result, nil
	}, output)
}

func runCommand(command *exec.Cmd, scan func(io.Reader, emit) (*core.Result, error), output emit) error {
	stdout, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		return err
	}
	result, scanErr := scan(stdout, output)
	if scanErr != nil && command.Cancel != nil {
		_ = command.Cancel()
	}
	waitErr := command.Wait()
	if scanErr != nil {
		return scanErr
	}
	if result == nil {
		reason := strings.TrimSpace(stderr.String())
		if reason == "" && waitErr != nil {
			reason = waitErr.Error()
		}
		if reason == "" {
			reason = "harness exited without an answer"
		}
		return output(core.Event{Kind: core.Done, Result: &core.Result{Failed: true, Reason: reason}})
	}
	if waitErr != nil && !result.Failed {
		result.Failed = true
		result.Reason = strings.TrimSpace(stderr.String())
		if result.Reason == "" {
			result.Reason = waitErr.Error()
		}
	}
	return output(core.Event{Kind: core.Done, Result: result})
}

func scan(reader io.Reader, output emit) (*core.Result, error) {
	var result *core.Result
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		var event line
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}
		switch event.Type {
		case "system":
			if event.Subtype == "init" {
				if err := output(core.Event{Kind: core.Started, Text: fmt.Sprintf("%s, %d tools", event.Model, len(event.Tools))}); err != nil {
					return nil, err
				}
			}
		case "assistant":
			for _, block := range event.Message.Content {
				switch block.Type {
				case "text":
					if text := strings.TrimSpace(block.Text); text != "" {
						if err := output(core.Event{Kind: core.Text, Text: text}); err != nil {
							return nil, err
						}
					}
				case "tool_use":
					if err := output(core.Event{Kind: core.Tool, Tool: block.Name, Text: summarize(block.Args)}); err != nil {
						return nil, err
					}
				}
			}
		case "rate_limit_event":
			if err := output(core.Event{Kind: core.Notice, Text: "rate limited, waiting"}); err != nil {
				return nil, err
			}
		case "result":
			result = &core.Result{Text: event.Result, Structured: event.StructuredOutput, Failed: event.IsError, Reason: event.Subtype}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func summarize(args json.RawMessage) string {
	var fields map[string]any
	if json.Unmarshal(args, &fields) != nil {
		return ""
	}
	for _, key := range []string{"command", "file_path", "pattern", "path", "url", "prompt", "description"} {
		if value, ok := fields[key].(string); ok && value != "" {
			return clip(value)
		}
	}
	return ""
}

func clip(text string) string {
	line := strings.TrimSpace(strings.SplitN(strings.TrimSpace(text), "\n", 2)[0])
	if len(line) > 90 {
		line = line[:90] + "…"
	}
	return line
}

func structured(text string) map[string]any {
	start, end := strings.Index(text, "{"), strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return nil
	}
	var shape map[string]any
	if json.Unmarshal([]byte(text[start:end+1]), &shape) != nil {
		return nil
	}
	return shape
}

var quotedModel = regexp.MustCompile(`'([A-Za-z0-9][A-Za-z0-9.\[\]-]*)'`)

func models(parent context.Context) []string {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	text, err := process.CommandContext(ctx, "claude", "--help").Output()
	if err != nil {
		return nil
	}
	block := string(text)
	if at := strings.Index(block, "--model <model>"); at >= 0 {
		block = block[at:]
	} else {
		return nil
	}
	seen, result := map[string]bool{}, []string{}
	for _, match := range quotedModel.FindAllStringSubmatch(block, -1) {
		if !seen[match[1]] {
			seen[match[1]] = true
			result = append(result, match[1])
		}
	}
	return result
}
