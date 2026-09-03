// ask-provider-text wraps a one-shot text command in the provider/v1 stream.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/template"

	"github.com/roshbhatia/ask/internal/process"
	core "github.com/roshbhatia/ask/internal/provider"
)

type options struct {
	inputMode  string
	modelFlag  string
	promptFlag string
	schemaFlag string
	schemaFile bool
	models     bool
	validate   bool
}

var inputPrompt = template.Must(template.New("input prompt").Parse(`{{.Prompt}}

Input:
{{.Input}}`))

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func run(arguments []string, stdin io.Reader, stdout io.Writer) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	flags := flag.NewFlagSet("ask-provider-text", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var opts options
	flags.StringVar(&opts.inputMode, "input-mode", "stdin", "send input through stdin or append it to the prompt")
	flags.StringVar(&opts.modelFlag, "model-flag", "", "underlying command flag for a selected model")
	flags.StringVar(&opts.promptFlag, "prompt-flag", "", "underlying command flag for the prompt")
	flags.StringVar(&opts.schemaFlag, "schema-flag", "", "underlying command flag for a JSON schema")
	flags.BoolVar(&opts.schemaFile, "schema-file", false, "pass the JSON schema through a temporary file")
	flags.BoolVar(&opts.models, "models", false, "return one model per command output line")
	flags.BoolVar(&opts.validate, "validate", false, "validate this adapter mapping without running the command")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	commandLine := flags.Args()
	if len(commandLine) == 0 {
		return errors.New("ask-provider-text requires a command after --")
	}

	var envelope core.Envelope
	if err := json.NewDecoder(stdin).Decode(&envelope); err != nil {
		return err
	}
	if envelope.Version != core.Protocol {
		return errors.New("unsupported provider protocol")
	}
	if opts.validate {
		if envelope.Action != core.ActionValidate {
			return errors.New("validation command received a non-validation request")
		}
		if err := validateOptions(opts, commandLine); err != nil {
			return err
		}
		return json.NewEncoder(stdout).Encode(core.ValidationResponse{Version: core.Protocol, Status: "ok"})
	}
	if opts.models {
		if envelope.Action != core.ActionModels {
			return errors.New("models command received a non-models request")
		}
		return runModels(ctx, commandLine, stdout)
	}
	if envelope.Action != core.ActionGenerate {
		return errors.New("unsupported provider action")
	}
	return runGenerate(ctx, opts, commandLine, envelope.Request, stdout)
}

func runModels(ctx context.Context, commandLine []string, stdout io.Writer) error {
	output, err := process.CommandContext(ctx, commandLine[0], commandLine[1:]...).Output()
	if err != nil {
		return err
	}
	models := modelNames(output)
	return json.NewEncoder(stdout).Encode(map[string]any{"version": core.Protocol, "models": models})
}

func validateOptions(opts options, commandLine []string) error {
	if opts.inputMode != "prompt" && opts.inputMode != "stdin" {
		return fmt.Errorf("unknown input mode %q", opts.inputMode)
	}
	if opts.schemaFile && opts.schemaFlag == "" {
		return errors.New("--schema-file requires --schema-flag")
	}
	if len(commandLine) == 0 || strings.TrimSpace(commandLine[0]) == "" {
		return errors.New("provider command is empty")
	}
	return nil
}

func modelNames(output []byte) []string {
	var value any
	if json.Unmarshal(output, &value) == nil {
		if models := modelsFromJSON(value); len(models) > 0 {
			return models
		}
	}
	models := make([]string, 0)
	for _, line := range strings.Split(string(output), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			models = append(models, line)
		}
	}
	return models
}

func modelsFromJSON(value any) []string {
	switch value := value.(type) {
	case map[string]any:
		if models, ok := value["models"]; ok {
			return modelsFromJSON(models)
		}
		for _, key := range []string{"id", "name"} {
			if model, ok := value[key].(string); ok && model != "" {
				return []string{model}
			}
		}
	case []any:
		models := make([]string, 0, len(value))
		for _, item := range value {
			models = append(models, modelsFromJSON(item)...)
		}
		return models
	case string:
		if value != "" {
			return []string{value}
		}
	}
	return nil
}

func runGenerate(ctx context.Context, opts options, commandLine []string, request core.Request, stdout io.Writer) error {
	arguments := append([]string(nil), commandLine[1:]...)
	if request.Model != "" {
		if opts.modelFlag == "" {
			return errors.New("this provider does not support a per-request model")
		}
		arguments = append(arguments, opts.modelFlag, request.Model)
	}
	cleanup := func() {}
	if opts.schemaFlag != "" && request.Schema != nil {
		value, remove, err := schemaArgument(request.Schema, opts.schemaFile)
		if err != nil {
			return err
		}
		cleanup = remove
		arguments = append(arguments, opts.schemaFlag, value)
	}
	defer cleanup()

	prompt := request.Prompt
	input := request.Input
	switch opts.inputMode {
	case "prompt":
		if len(input) > 0 {
			var rendered strings.Builder
			if err := inputPrompt.Execute(&rendered, struct {
				Prompt string
				Input  string
			}{Prompt: prompt, Input: string(input)}); err != nil {
				return err
			}
			prompt, input = rendered.String(), ""
		}
	case "stdin":
	default:
		return fmt.Errorf("unknown input mode %q", opts.inputMode)
	}
	if opts.promptFlag != "" {
		arguments = append(arguments, opts.promptFlag)
	}
	arguments = append(arguments, prompt)

	command := process.CommandContext(ctx, commandLine[0], arguments...)
	command.Dir = request.Dir
	command.Stdin = strings.NewReader(input)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	encoder := json.NewEncoder(stdout)
	if err := encoder.Encode(core.Event{Version: core.Protocol, Kind: core.Started, Text: request.Model}); err != nil {
		return err
	}
	output, runErr := command.Output()
	result := &core.Result{Text: strings.TrimSpace(string(output))}
	if runErr != nil || result.Text == "" {
		result.Failed = true
		result.Reason = strings.TrimSpace(stderr.String())
		if result.Reason == "" && runErr != nil {
			result.Reason = runErr.Error()
		}
		if result.Reason == "" {
			result.Reason = "command exited without an answer"
		}
	} else {
		if request.Schema != nil {
			result.Structured = structured(result.Text)
			if result.Structured == nil {
				result.Failed = true
				result.Reason = "answered outside the shape --schema asked for"
			}
		}
		if err := encoder.Encode(core.Event{Version: core.Protocol, Kind: core.Text, Text: result.Text}); err != nil {
			return err
		}
	}
	return encoder.Encode(core.Event{Version: core.Protocol, Kind: core.Done, Result: result})
}

func schemaArgument(shape map[string]any, asFile bool) (string, func(), error) {
	encoded, err := json.Marshal(shape)
	if err != nil {
		return "", func() {}, err
	}
	if !asFile {
		return string(encoded), func() {}, nil
	}
	file, err := os.CreateTemp("", "ask-schema-*.json")
	if err != nil {
		return "", func() {}, err
	}
	remove := func() { _ = os.Remove(file.Name()) }
	if _, err := file.Write(encoded); err != nil {
		_ = file.Close()
		remove()
		return "", func() {}, err
	}
	if err := file.Close(); err != nil {
		remove()
		return "", func() {}, err
	}
	return file.Name(), remove, nil
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
