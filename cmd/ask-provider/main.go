// ask-provider adapts proven harness JSON streams to the provider/v1 protocol.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	core "github.com/roshbhatia/ask/internal/provider"
)

const offShape = "answered outside the shape --schema asked for"

type emit func(core.Event) error

func send(encoder *json.Encoder) emit {
	return func(event core.Event) error {
		event.Version = core.Protocol
		return encoder.Encode(event)
	}
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

func runCommand(command *exec.Cmd, scan func(io.Reader, emit) *core.Result, output emit) error {
	stdout, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		return err
	}
	result := scan(stdout, output)
	waitErr := command.Wait()
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

type claudeLine struct {
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

func scanClaude(reader io.Reader, output emit) *core.Result {
	var result *core.Result
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		var event claudeLine
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}
		switch event.Type {
		case "system":
			if event.Subtype == "init" {
				_ = output(core.Event{Kind: core.Started, Text: fmt.Sprintf("%s, %d tools", event.Model, len(event.Tools))})
			}
		case "assistant":
			for _, block := range event.Message.Content {
				switch block.Type {
				case "text":
					if text := strings.TrimSpace(block.Text); text != "" {
						_ = output(core.Event{Kind: core.Text, Text: text})
					}
				case "tool_use":
					_ = output(core.Event{Kind: core.Tool, Tool: block.Name, Text: summarize(block.Args)})
				}
			}
		case "rate_limit_event":
			_ = output(core.Event{Kind: core.Notice, Text: "rate limited, waiting"})
		case "result":
			result = &core.Result{Text: event.Result, Structured: event.StructuredOutput, Failed: event.IsError, Reason: event.Subtype}
		}
	}
	return result
}

func runClaude(ctx context.Context, request core.Request, output emit) error {
	binary, err := exec.LookPath("claude")
	if err != nil {
		return output(core.Event{Kind: core.Done, Result: &core.Result{Failed: true, Reason: "claude is not on PATH; install Claude Code or pass --provider"}})
	}
	args := []string{"--print", "--output-format", "stream-json", "--verbose", "--no-session-persistence"}
	if request.Model != "" {
		args = append(args, "--model", request.Model)
	}
	if request.Schema != nil {
		encoded, marshalErr := json.Marshal(request.Schema)
		if marshalErr != nil {
			return marshalErr
		}
		args = append(args, "--json-schema", string(encoded))
	}
	args = append(args, "--", request.Prompt)
	command := exec.CommandContext(ctx, binary, args...)
	command.Dir, command.Stdin = request.Dir, bytes.NewReader(request.Input)
	return runCommand(command, func(reader io.Reader, output emit) *core.Result {
		result := scanClaude(reader, output)
		if result != nil && request.Schema != nil && !result.Failed && result.Structured == nil {
			result.Structured = structured(result.Text)
			if result.Structured == nil {
				result.Failed, result.Reason = true, offShape
			}
		}
		return result
	}, output)
}

type codexItem struct {
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

type codexLine struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Error   struct {
		Message string `json:"message"`
	} `json:"error"`
	Item codexItem `json:"item"`
}

func toolOf(item codexItem) (string, string, bool) {
	switch item.Type {
	case "command_execution":
		return "shell", clip(item.Command), true
	case "file_change":
		paths := make([]string, 0, len(item.Changes))
		for _, change := range item.Changes {
			paths = append(paths, change.Path)
		}
		return "edit", clip(strings.Join(paths, " ")), true
	case "mcp_tool_call":
		name := item.Server
		if item.Tool != "" {
			name += "." + item.Tool
		}
		return "mcp", clip(name), true
	case "web_search":
		return "search", clip(item.Query), true
	default:
		return "", "", false
	}
}

func scanCodex(reader io.Reader, model string, output emit) *core.Result {
	var answer, failure string
	announced := map[string]bool{}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		var event codexLine
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}
		switch event.Type {
		case "thread.started":
			if model == "" {
				model = "default model"
			}
			_ = output(core.Event{Kind: core.Started, Text: model})
		case "item.started", "item.completed":
			done, item := event.Type == "item.completed", event.Item
			switch {
			case item.Type == "agent_message" && done:
				if text := strings.TrimSpace(item.Text); text != "" {
					answer = text
					_ = output(core.Event{Kind: core.Text, Text: text})
				}
			case item.Type == "reasoning" && done:
				if text := strings.TrimSpace(item.Text); text != "" {
					_ = output(core.Event{Kind: core.Text, Text: text})
				}
			case item.Type == "error" && done:
				_ = output(core.Event{Kind: core.Notice, Text: clip(item.Message)})
			case !announced[item.ID]:
				if name, text, ok := toolOf(item); ok {
					announced[item.ID] = true
					_ = output(core.Event{Kind: core.Tool, Tool: name, Text: text})
				}
			}
		case "turn.failed":
			failure = event.Error.Message
		case "error":
			_ = output(core.Event{Kind: core.Notice, Text: clip(event.Message)})
		}
	}
	result := &core.Result{Text: answer}
	switch {
	case failure != "":
		result.Failed, result.Reason = true, failure
	case answer == "":
		return nil
	}
	return result
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

func runCodex(ctx context.Context, request core.Request, output emit) error {
	binary, err := exec.LookPath("codex")
	if err != nil {
		return output(core.Event{Kind: core.Done, Result: &core.Result{Failed: true, Reason: "codex is not on PATH; install the Codex CLI or pass --provider"}})
	}
	var schemaPath string
	if request.Schema != nil {
		if schemaPath, err = schemaFile(request.Schema); err != nil {
			return err
		}
		defer func() { _ = os.Remove(schemaPath) }()
	}
	args := codexArgs(request, schemaPath)
	command := exec.CommandContext(ctx, binary, args...)
	command.Dir, command.Stdin = request.Dir, bytes.NewReader(request.Input)
	return runCommand(command, func(reader io.Reader, output emit) *core.Result {
		result := scanCodex(reader, request.Model, output)
		if result != nil && request.Schema != nil && !result.Failed {
			result.Structured = structured(result.Text)
			if result.Structured == nil {
				result.Failed, result.Reason = true, offShape
			}
		}
		return result
	}, output)
}

func codexArgs(request core.Request, schemaPath string) []string {
	args := []string{"exec", "--json", "--sandbox", "read-only", "--ephemeral", "--skip-git-repo-check"}
	if request.Model != "" {
		args = append(args, "--model", request.Model)
	}
	if schemaPath != "" {
		args = append(args, "--output-schema", schemaPath)
	}
	return append(args, "--", request.Prompt)
}

var quotedModel = regexp.MustCompile(`'([A-Za-z0-9][A-Za-z0-9.\[\]-]*)'`)

func claudeModels() []string {
	binary, err := exec.LookPath("claude")
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	text, err := exec.CommandContext(ctx, binary, "--help").Output()
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

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "ask-provider requires claude or codex")
		os.Exit(2)
	}
	var request core.Envelope
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if request.Version != core.Protocol {
		fmt.Fprintln(os.Stderr, "unsupported provider protocol")
		os.Exit(2)
	}
	encoder, output := json.NewEncoder(os.Stdout), send(json.NewEncoder(os.Stdout))
	if request.Action == core.ActionModels {
		models := []string{}
		if os.Args[1] == "claude" {
			models = claudeModels()
		}
		if err := encoder.Encode(map[string]any{"version": core.Protocol, "models": models}); err != nil {
			os.Exit(1)
		}
		return
	}
	if request.Action != core.ActionGenerate {
		fmt.Fprintln(os.Stderr, "unsupported provider action")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "claude":
		err = runClaude(context.Background(), request.Request, output)
	case "codex":
		err = runCodex(context.Background(), request.Request, output)
	default:
		err = fmt.Errorf("unknown harness %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
