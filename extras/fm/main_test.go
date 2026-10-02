package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/roshbhatia/ask/extras/internal/textadapter"
	core "github.com/roshbhatia/ask/internal/provider"
	"go.yaml.in/yaml/v3"
)

func TestFMProcess(t *testing.T) {
	if os.Getenv("ASK_FM_HELPER") == "" {
		return
	}
	args := os.Args
	if slices.Contains(args, "--list-models") {
		_, _ = os.Stdout.WriteString("system\n")
		os.Exit(0)
	}
	for _, arg := range []string{"respond", "--no-stream", "--model", "system", "--text"} {
		if !slices.Contains(args, arg) {
			os.Exit(2)
		}
	}
	prompt := args[len(args)-1]
	for _, want := range []string{"--leading prompt", "Input:\nMira has 3 apples.", "Return one JSON object only"} {
		if !strings.Contains(prompt, want) {
			os.Exit(2)
		}
	}
	input, err := io.ReadAll(os.Stdin)
	if err != nil || len(input) != 0 {
		os.Exit(2)
	}
	if os.Getenv("ASK_FM_FAIL") == "1" {
		_, _ = os.Stderr.WriteString("System model unavailable")
		os.Exit(1)
	}
	_, _ = os.Stdout.WriteString(`{"name":"Mira","count":3}`)
	os.Exit(0)
}

func TestFMManifest(t *testing.T) {
	raw, err := os.ReadFile("provider.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Actions map[string]struct {
			Argv []string `yaml:"argv"`
		} `yaml:"actions"`
		Defaults struct {
			Model string `yaml:"model"`
			Light string `yaml:"light"`
		} `yaml:"defaults"`
	}
	if err := yaml.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Defaults.Model != "system" || manifest.Defaults.Light != "system" {
		t.Fatal("model roles must select system")
	}
	t.Setenv("ASK_FM_HELPER", "1")
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "generation", true: "unavailable"}[failed], func(t *testing.T) {
			if failed {
				t.Setenv("ASK_FM_FAIL", "1")
			}
			args := slices.Clone(manifest.Actions[core.ActionGenerate].Argv)
			separator := slices.Index(args, "--")
			if separator < 0 || args[separator+1] != "/usr/bin/fm" {
				t.Fatal("manifest must invoke the OS CLI")
			}
			args = append(args[:separator+1], append([]string{os.Args[0], "-test.run=TestFMProcess", "--"}, args[separator+2:]...)...)
			request := core.Envelope{Version: core.Protocol, Action: core.ActionGenerate, Request: core.Request{
				Prompt: "--leading prompt", Input: "Mira has 3 apples.", Model: "system", Dir: t.TempDir(),
				Schema: map[string]any{"type": "object"},
			}}
			encoded, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			if err := textadapter.Run(args, bytes.NewReader(encoded), &output); err != nil {
				t.Fatal(err)
			}
			decoder := json.NewDecoder(&output)
			var result *core.Result
			for decoder.More() {
				var event core.Event
				if err := decoder.Decode(&event); err != nil {
					t.Fatal(err)
				}
				if event.Result != nil {
					result = event.Result
				}
			}
			if result == nil || result.Failed != failed {
				t.Fatalf("result = %#v", result)
			}
			if failed && !strings.Contains(result.Reason, "System model unavailable") {
				t.Fatalf("reason = %s", result.Reason)
			}
			if !failed && result.Structured["name"] != "Mira" {
				t.Fatalf("structured = %#v", result.Structured)
			}
		})
	}
	for _, action := range []string{core.ActionModels, core.ActionValidate} {
		encoded, err := json.Marshal(core.Envelope{Version: core.Protocol, Action: action})
		if err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		args := manifest.Actions[action].Argv
		if action == core.ActionModels {
			args = []string{"--models", "--", os.Args[0], "-test.run=TestFMProcess", "--", "--list-models"}
		}
		if err := textadapter.Run(args, bytes.NewReader(encoded), &output); err != nil {
			t.Fatal(err)
		}
		if !json.Valid(output.Bytes()) {
			t.Fatalf("invalid protocol response: %s", &output)
		}
	}
}
