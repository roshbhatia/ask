// ask-provider-hermes adapts one-shot Hermes output to provider/v1.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const (
	protocol       = "provider/v1"
	actionGenerate = "inference.generate"
	actionValidate = "provider.validate"
)

type request struct {
	Prompt string `json:"prompt"`
	Input  string `json:"input,omitempty"`
	Model  string `json:"model,omitempty"`
	Dir    string `json:"directory"`
}

type envelope struct {
	Version string  `json:"version"`
	Action  string  `json:"action"`
	Request request `json:"request"`
}

type result struct {
	Text   string `json:"text,omitempty"`
	Failed bool   `json:"failed,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type event struct {
	Version string  `json:"version"`
	Kind    string  `json:"type"`
	Text    string  `json:"text,omitempty"`
	Result  *result `json:"result,omitempty"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func run() error {
	var message envelope
	if err := json.NewDecoder(os.Stdin).Decode(&message); err != nil {
		return err
	}
	if message.Version != protocol {
		return errors.New("unsupported provider protocol")
	}
	if message.Action == actionValidate {
		return json.NewEncoder(os.Stdout).Encode(map[string]string{"version": protocol, "status": "ok"})
	}
	if message.Action != actionGenerate {
		return errors.New("unsupported provider action")
	}
	return generate(message.Request)
}

func generate(one request) error {
	arguments := []string{"-z"}
	if one.Model != "" {
		arguments = append(arguments, "--model", one.Model)
	}
	arguments = append(arguments, one.Prompt)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	command := commandContext(ctx, "hermes", arguments...)
	command.Dir = one.Dir
	command.Stdin = strings.NewReader(one.Input)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	encoder := json.NewEncoder(os.Stdout)
	if err := encoder.Encode(event{Version: protocol, Kind: "started", Text: one.Model}); err != nil {
		return err
	}
	output, runErr := command.Output()
	answer := strings.TrimSpace(string(output))
	done := &result{Text: answer}
	if runErr != nil || answer == "" {
		done.Failed = true
		done.Reason = strings.TrimSpace(stderr.String())
		if done.Reason == "" && runErr != nil {
			done.Reason = runErr.Error()
		}
		if done.Reason == "" {
			done.Reason = "Hermes exited without an answer"
		}
	} else if err := encoder.Encode(event{Version: protocol, Kind: "text", Text: answer}); err != nil {
		return err
	}
	return encoder.Encode(event{Version: protocol, Kind: "result", Result: done})
}

func commandContext(ctx context.Context, name string, arguments ...string) *exec.Cmd {
	command := exec.CommandContext(ctx, name, arguments...)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.WaitDelay = 2 * time.Second
	command.Cancel = func() error {
		if command.Process == nil {
			return os.ErrProcessDone
		}
		pid := command.Process.Pid
		if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
			if errors.Is(err, syscall.ESRCH) {
				return os.ErrProcessDone
			}
			return err
		}
		go func() {
			time.Sleep(2 * time.Second)
			_ = syscall.Kill(-pid, syscall.SIGKILL)
		}()
		return nil
	}
	return command
}
