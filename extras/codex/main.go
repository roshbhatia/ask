// ask-provider-codex adapts Codex's JSON stream to provider/v1.
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
	"strings"
	"syscall"

	"github.com/roshbhatia/ask/internal/process"
	core "github.com/roshbhatia/ask/internal/provider"
)

const offShape = "answered outside the shape --schema asked for"

type emit func(core.Event) error

type item struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Text    string `json:"text"`
	Message string `json:"message"`
	Command string `json:"command"`
	Server  string `json:"server"`
	Tool    string `json:"tool"`
	Query   string `json:"query"`
	Changes []struct {
		Path string `json:"path"`
	} `json:"changes"`
}

type line struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Error   struct {
		Message string `json:"message"`
	} `json:"error"`
	Item item `json:"item"`
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
		if err := json.NewEncoder(os.Stdout).Encode(map[string]any{"version": core.Protocol, "models": []string{}}); err != nil {
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
	var schemaPath string
	if request.Schema != nil {
		var err error
		if schemaPath, err = schemaFile(request.Schema); err != nil {
			return err
		}
		defer func() { _ = os.Remove(schemaPath) }()
	}
	command := process.CommandContext(ctx, "codex", arguments(request, schemaPath)...)
	command.Dir, command.Stdin = request.Dir, strings.NewReader(request.Input)
	return runCommand(command, func(reader io.Reader, output emit) (*core.Result, error) {
		result, err := scan(reader, request.Model, output)
		if err != nil {
			return nil, err
		}
		if result != nil && request.Schema != nil && !result.Failed {
			result.Structured = structured(result.Text)
			if result.Structured == nil {
				result.Failed, result.Reason = true, offShape
			}
		}
		return result, nil
	}, output)
}

func arguments(request core.Request, schemaPath string) []string {
	args := []string{"exec", "--json", "--sandbox", "read-only", "--ephemeral", "--skip-git-repo-check"}
	if request.Model != "" {
		args = append(args, "--model", request.Model)
	}
	if schemaPath != "" {
		args = append(args, "--output-schema", schemaPath)
	}
	return append(args, "--", request.Prompt)
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

func scan(reader io.Reader, model string, output emit) (*core.Result, error) {
	var answer, failure string
	announced := map[string]bool{}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		var event line
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}
		switch event.Type {
		case "thread.started":
			if model == "" {
				model = "default model"
			}
			if err := output(core.Event{Kind: core.Started, Text: model}); err != nil {
				return nil, err
			}
		case "item.started", "item.completed":
			done, current := event.Type == "item.completed", event.Item
			switch {
			case current.Type == "agent_message" && done:
				if text := strings.TrimSpace(current.Text); text != "" {
					answer = text
					if err := output(core.Event{Kind: core.Text, Text: text}); err != nil {
						return nil, err
					}
				}
			case current.Type == "reasoning" && done:
				if text := strings.TrimSpace(current.Text); text != "" {
					if err := output(core.Event{Kind: core.Text, Text: text}); err != nil {
						return nil, err
					}
				}
			case current.Type == "error" && done:
				if err := output(core.Event{Kind: core.Notice, Text: clip(current.Message)}); err != nil {
					return nil, err
				}
			case !announced[current.ID]:
				if name, text, ok := toolOf(current); ok {
					announced[current.ID] = true
					if err := output(core.Event{Kind: core.Tool, Tool: name, Text: text}); err != nil {
						return nil, err
					}
				}
			}
		case "turn.failed":
			failure = event.Error.Message
		case "error":
			if err := output(core.Event{Kind: core.Notice, Text: clip(event.Message)}); err != nil {
				return nil, err
			}
		}
	}
	result := &core.Result{Text: answer}
	switch {
	case failure != "":
		result.Failed, result.Reason = true, failure
	case answer == "":
		return nil, nil
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func toolOf(current item) (string, string, bool) {
	switch current.Type {
	case "command_execution":
		return "shell", clip(current.Command), true
	case "file_change":
		paths := make([]string, 0, len(current.Changes))
		for _, change := range current.Changes {
			paths = append(paths, change.Path)
		}
		return "edit", clip(strings.Join(paths, " ")), true
	case "mcp_tool_call":
		name := current.Server
		if current.Tool != "" {
			name += "." + current.Tool
		}
		return "mcp", clip(name), true
	case "web_search":
		return "search", clip(current.Query), true
	default:
		return "", "", false
	}
}

func schemaFile(shape map[string]any) (string, error) {
	encoded, err := json.Marshal(shape)
	if err != nil {
		return "", err
	}
	file, err := os.CreateTemp("", "ask-schema-*.json")
	if err != nil {
		return "", err
	}
	if _, err := file.Write(encoded); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return "", err
	}
	return file.Name(), file.Close()
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

func clip(text string) string {
	line := strings.TrimSpace(strings.SplitN(strings.TrimSpace(text), "\n", 2)[0])
	if len(line) > 90 {
		line = line[:90] + "…"
	}
	return line
}
