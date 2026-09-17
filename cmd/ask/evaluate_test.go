package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/roshbhatia/ask/internal/evaluation"
	"github.com/roshbhatia/ask/internal/provider"
	"github.com/roshbhatia/ask/internal/schema"
	"github.com/roshbhatia/ask/internal/templates"
	shared "github.com/roshbhatia/go-utils/provider"
	"go.yaml.in/yaml/v3"
)

func TestEvaluationProviderProcess(t *testing.T) {
	mode := os.Getenv("ASK_EVALUATION_TEST")
	if mode == "" {
		return
	}
	var request struct {
		Action  string `json:"action"`
		Request struct {
			Input string `json:"input"`
			State any    `json:"state"`
			Model string `json:"model"`
		} `json:"request"`
	}
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		os.Exit(2)
	}
	if request.Action == provider.ActionValidate {
		json.NewEncoder(os.Stdout).Encode(provider.ValidationResponse{Version: provider.Protocol, Status: "ok"})
		os.Exit(0)
	}
	if mode == "slow" {
		time.Sleep(time.Second)
	}
	if mode == "native" {
		p := 0.01
		json.NewEncoder(os.Stdout).Encode(provider.EvaluationResponse{Version: provider.Protocol, Answers: map[string]evaluation.Answer{"ok": {Value: false, Probability: &p}}, Metadata: map[string]any{"model": request.Request.Model, "state": request.Request.State}})
	} else {
		result := provider.Result{Structured: map[string]any{"ok": false}}
		if mode == "bad" {
			result.Structured["ok"] = "false"
		}
		if mode == "generate" {
			result = provider.Result{Text: "answer"}
		}
		json.NewEncoder(os.Stdout).Encode(provider.Event{Version: provider.Protocol, Kind: provider.Done, Result: &result})
	}
	os.Exit(0)
}

func installEvaluationProvider(t *testing.T, mode string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_DATA_DIRS", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("ASK_PROVIDER_PATH", "")
	t.Setenv("ASK_CONFIG", "")
	t.Setenv("ASK_PROVIDER", "wrong-generation-provider")
	t.Setenv("ASK_PROVIDER_DEFAULT", "wrong-generation-provider")
	t.Setenv("ASK_EVALUATION_PROVIDER", "sample")
	t.Setenv("ASK_EVALUATION_MODEL", "classifier")
	dir := filepath.Join(root, "ask", "providers", "sample")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	action := provider.ActionGenerate
	if mode == "native" {
		action = provider.ActionEvaluate
	}
	manifest := shared.Manifest{Version: provider.Protocol, Name: "sample", Description: "evaluation test", Command: []string{os.Args[0], "-test.run=TestEvaluationProviderProcess"}, Actions: map[string]shared.Action{action: {Description: "inference", Env: map[string]string{"ASK_EVALUATION_TEST": mode}}, provider.ActionValidate: {Description: "validate", Env: map[string]string{"ASK_EVALUATION_TEST": mode}}}}
	raw, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "provider.yaml"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestEvaluationPipeline(t *testing.T) {
	for _, mode := range []string{"structured", "native", "bad", "slow"} {
		t.Run(mode, func(t *testing.T) {
			installEvaluationProvider(t, mode)
			opts := evaluationOptions{method: "auto", input: "json", timeout: 500 * time.Millisecond, maxRepairs: 0, booleans: []string{"ok=Did the answer follow the request?"}}
			if mode != "slow" {
				opts.timeout = 5 * time.Second
			}
			var out bytes.Buffer
			err := evaluateInput(context.Background(), opts, strings.NewReader(`{"prompt":"say no","answer":"no"}`), &out)
			if mode == "bad" || mode == "slow" {
				if err == nil || out.Len() != 0 {
					t.Fatalf("err=%v output=%s", err, &out)
				}
				if mode == "slow" && !errors.Is(err, context.DeadlineExceeded) {
					t.Fatal(err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var result evaluation.Result
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Method != mode || result.Model != "classifier" || result.Answers["ok"].Value != false {
				t.Fatalf("result=%+v", result)
			}
			if mode == "structured" && result.Answers["ok"].Probability != nil {
				t.Fatal("invented probability")
			}
			if mode == "native" {
				if result.Answers["ok"].Probability == nil || *result.Answers["ok"].Probability != 0.01 {
					t.Fatal("lost probability")
				}
				if _, err := provider.Find("sample"); err == nil {
					t.Fatal("evaluation-only provider accepted for generation")
				}
				reports, err := provider.Validate("sample", t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				for _, report := range reports {
					if !report.OK() {
						t.Fatalf("validation=%+v", report)
					}
				}
			}
		})
	}
}

func TestQuestionsAndRubrics(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	opts := evaluationOptions{booleans: []string{"ok=Is it ready?"}, choices: []string{"kind=Which kind?"}, labels: []string{"kind=fix,feature,other"}, scores: []string{"impact=How much?"}, levels: []string{"impact=low,medium,high"}}
	questions, err := opts.buildQuestions()
	if err != nil {
		t.Fatal(err)
	}
	if err := templates.SaveRubric(templates.Rubric{Name: "review", Questions: questions}); err != nil {
		t.Fatal(err)
	}
	loaded, err := (evaluationOptions{rubric: "review"}).buildQuestions()
	if err != nil || len(loaded) != 3 {
		t.Fatalf("loaded=%v err=%v", loaded, err)
	}
	for _, invalid := range []evaluationOptions{
		{rubric: "review", booleans: []string{"ok=Duplicate?"}}, {booleans: []string{"ok=Yes?", "ok=No?"}},
		{choices: opts.choices, labels: []string{"kind=fix,fix"}}, {labels: opts.labels}, {scores: opts.scores, levels: []string{"impact=low"}},
	} {
		if _, err := invalid.buildQuestions(); err == nil {
			t.Fatalf("accepted %+v", invalid)
		}
	}
}

func TestGenerationEnvelopeFeedsEvaluation(t *testing.T) {
	installEvaluationProvider(t, "generate")
	generation := exec.Command(os.Args[0], "-test.run=TestAskHelperProcess", "--", "-p", "sample", "-m", "writer", "--envelope", "--quiet", "say no")
	generation.Env = append(os.Environ(), "ASK_TEST_HELPER=1")
	generation.Stdin = strings.NewReader("source material")
	raw, err := generation.Output()
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["prompt"] != "say no" || envelope["input"] != "source material" || envelope["answer"] != "answer" || envelope["model"] != "writer" {
		t.Fatalf("envelope=%v", envelope)
	}
	installEvaluationProvider(t, "native")
	classification := exec.Command(os.Args[0], "-test.run=TestAskHelperProcess", "--", "evaluate", "--boolean", "ok=Did the answer follow the prompt?")
	classification.Env = append(os.Environ(), "ASK_TEST_HELPER=1")
	classification.Stdin = bytes.NewReader(raw)
	output, err := classification.Output()
	if err != nil {
		t.Fatal(err)
	}
	var result evaluation.Result
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatal(err)
	}
	state := result.Metadata["state"].(map[string]any)
	if state["prompt"] != "say no" || state["answer"] != "answer" || result.Model != "classifier" {
		t.Fatalf("result=%+v", result)
	}
}

type deadlineProvider struct{ deadlines []time.Time }

func (p *deadlineProvider) Name() string { return "deadline" }
func (p *deadlineProvider) Run(ctx context.Context, _ provider.Request) (<-chan provider.Event, error) {
	deadline, _ := ctx.Deadline()
	p.deadlines = append(p.deadlines, deadline)
	events := make(chan provider.Event, 1)
	events <- provider.Event{Kind: provider.Done, Result: &provider.Result{Structured: map[string]any{"ok": "wrong"}}}
	close(events)
	return events, nil
}

func TestRepairAttemptsShareDeadline(t *testing.T) {
	p := &deadlineProvider{}
	shape, err := schema.Resolve("ok:bool")
	if err != nil {
		t.Fatal(err)
	}
	_, err = converse(provider.Request{}, shape, options{timeout: time.Second, quiet: true}, p)
	if err == nil || len(p.deadlines) != rounds {
		t.Fatalf("deadlines=%v err=%v", p.deadlines, err)
	}
	for _, deadline := range p.deadlines {
		if !deadline.Equal(p.deadlines[0]) {
			t.Fatal("deadline restarted for repair")
		}
	}
}
