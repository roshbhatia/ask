// Package provider discovers and invokes external inference providers.
package provider

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
	"path/filepath"
	"slices"
	"strings"
	"time"

	shared "github.com/roshbhatia/go-utils/provider"
)

const (
	ActionGenerate = "inference.generate"
	ActionModels   = "inference.models"
	Protocol       = "provider/v1"
)

type Request struct {
	Prompt string         `json:"prompt"`
	Input  []byte         `json:"input,omitempty"`
	Model  string         `json:"model,omitempty"`
	Schema map[string]any `json:"schema,omitempty"`
	Dir    string         `json:"directory"`
}

type Kind string

const (
	Started Kind = "started"
	Text    Kind = "text"
	Tool    Kind = "tool"
	Notice  Kind = "notice"
	Done    Kind = "result"
)

type Event struct {
	Version string  `json:"version,omitempty"`
	Kind    Kind    `json:"type"`
	Text    string  `json:"text,omitempty"`
	Tool    string  `json:"tool,omitempty"`
	Result  *Result `json:"result,omitempty"`
}

type Result struct {
	Text       string         `json:"text,omitempty"`
	Structured map[string]any `json:"structured,omitempty"`
	Failed     bool           `json:"failed,omitempty"`
	Reason     string         `json:"reason,omitempty"`
}

type Envelope struct {
	Version string  `json:"version"`
	Action  string  `json:"action"`
	Request Request `json:"request"`
}

type modelResponse struct {
	Version string   `json:"version"`
	Models  []string `json:"models"`
}

type Provider interface {
	Name() string
	Run(context.Context, Request) (<-chan Event, error)
}

type Info struct {
	Name   string
	Short  string
	Blurb  string
	Binary string

	manifest shared.Manifest
}

func (i Info) New() Provider { return commandProvider{manifest: i.manifest} }

func (i Info) Ready() bool {
	return (shared.Validator{}).Validate(i.manifest, "").OK()
}

func (i Info) Models() []string {
	if _, ok := i.manifest.Actions[ActionModels]; !ok || !i.Ready() {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	plan, err := i.manifest.Render(ActionModels, map[string]any{})
	if err != nil {
		return nil
	}
	value := modelResponse{}
	if err := runJSON(ctx, plan, Envelope{Version: Protocol, Action: ActionModels}, &value); err != nil || value.Version != Protocol {
		return nil
	}
	return slices.Clone(value.Models)
}

func compatAlias(name string) string {
	switch name {
	case "claude":
		return "cld"
	case "codex":
		return "cdx"
	default:
		return name
	}
}

func providerDirectory() string {
	if configured := strings.TrimSpace(os.Getenv("ASK_PROVIDERS_DIRECTORY")); configured != "" {
		return configured
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "ask", "providers")
}

func providerRoots() []string {
	result := []string{providerDirectory()}
	for _, path := range filepath.SplitList(os.Getenv("ASK_PROVIDER_PATH")) {
		if path != "" && !slices.Contains(result, path) {
			result = append(result, path)
		}
	}
	if executable, err := os.Executable(); err == nil {
		packaged := filepath.Join(filepath.Dir(executable), "providers")
		if !slices.Contains(result, packaged) {
			result = append(result, packaged)
		}
	}
	return result
}

func directories() ([]string, error) {
	var result []string
	for _, root := range providerRoots() {
		result = append(result, root)
		entries, err := os.ReadDir(root)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read provider directory %s: %w", root, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				result = append(result, filepath.Join(root, entry.Name()))
			}
		}
	}
	return result, nil
}

func Discover() ([]Info, error) {
	seen := map[string]bool{}
	aliases := map[string]string{}
	var result []Info
	directories, err := directories()
	if err != nil {
		return nil, err
	}
	for _, directory := range directories {
		loaded, err := shared.Discover(directory)
		if err != nil {
			return nil, err
		}
		for _, item := range loaded {
			manifest := item.Manifest
			if _, ok := manifest.Actions[ActionGenerate]; !ok {
				continue
			}
			if seen[manifest.Name] {
				continue
			}
			alias := compatAlias(manifest.Name)
			if owner, exists := aliases[alias]; exists && owner != manifest.Name {
				return nil, fmt.Errorf("provider alias %q is shared by %q and %q", alias, owner, manifest.Name)
			}
			seen[manifest.Name], aliases[alias] = true, manifest.Name
			result = append(result, Info{Name: manifest.Name, Short: alias, Blurb: manifest.Description, Binary: manifest.Command[0], manifest: manifest})
		}
	}
	slices.SortFunc(result, func(a, b Info) int { return strings.Compare(a.Name, b.Name) })
	return result, nil
}

func Known() []Info {
	providers, _ := Discover()
	return providers
}

func Lookup(name string) (Info, bool) {
	providers, err := Discover()
	if err != nil {
		return Info{}, false
	}
	for _, one := range providers {
		if name == one.Name || name == one.Short {
			return one, true
		}
	}
	return Info{}, false
}

func Names() []string {
	providers, _ := Discover()
	names := make([]string, 0, len(providers)*2)
	for _, one := range providers {
		names = append(names, one.Name)
		if one.Short != one.Name {
			names = append(names, one.Short)
		}
	}
	return names
}

func Find(name string) (Provider, error) {
	if name == "" {
		return nil, errors.New("say which agent to run, with -p")
	}
	providers, err := Discover()
	if err != nil {
		return nil, err
	}
	for _, one := range providers {
		if name == one.Name || name == one.Short {
			return one.New(), nil
		}
	}
	return nil, fmt.Errorf("unknown provider %q, known: %s", name, strings.Join(Names(), ", "))
}

type commandProvider struct{ manifest shared.Manifest }

func (p commandProvider) Name() string { return p.manifest.Name }

func (p commandProvider) Run(ctx context.Context, request Request) (<-chan Event, error) {
	plan, err := p.manifest.Render(ActionGenerate, request)
	if err != nil {
		return nil, err
	}
	cancel := func() {}
	if plan.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, plan.Timeout)
	}
	payload, err := json.Marshal(Envelope{Version: Protocol, Action: ActionGenerate, Request: request})
	if err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, plan.Argv[0], plan.Argv[1:]...)
	command.Dir = request.Dir
	command.Env = append(os.Environ(), environment(plan.Env)...)
	command.Stdin = bytes.NewReader(payload)
	stdout, err := command.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		cancel()
		return nil, err
	}
	events := make(chan Event, 64)
	go func() {
		defer close(events)
		defer cancel()
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
		answered := false
		protocolError := ""
		for scanner.Scan() {
			var event Event
			if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
				protocolError = "provider returned invalid JSON: " + err.Error()
				continue
			}
			if err := validateEvent(event); err != nil {
				protocolError = err.Error()
				continue
			}
			if event.Kind == Done {
				answered = true
			}
			events <- event
		}
		waitErr := command.Wait()
		if protocolError != "" {
			events <- Event{Version: Protocol, Kind: Done, Result: &Result{Failed: true, Reason: protocolError}}
			return
		}
		if answered {
			return
		}
		reason := strings.TrimSpace(stderr.String())
		if reason == "" && scanner.Err() != nil {
			reason = scanner.Err().Error()
		}
		if reason == "" && waitErr != nil {
			reason = waitErr.Error()
		}
		if reason == "" {
			reason = p.manifest.Name + " exited without an answer"
		}
		events <- Event{Version: Protocol, Kind: Done, Result: &Result{Failed: true, Reason: reason}}
	}()
	return events, nil
}

func validateEvent(event Event) error {
	if event.Version != Protocol {
		return fmt.Errorf("provider returned protocol %q, want %q", event.Version, Protocol)
	}
	switch event.Kind {
	case Started, Text, Tool, Notice:
		if event.Result != nil {
			return fmt.Errorf("provider event %q must not contain a result", event.Kind)
		}
	case Done:
		if event.Result == nil {
			return errors.New("provider result event has no result")
		}
	default:
		return fmt.Errorf("provider returned unknown event type %q", event.Kind)
	}
	return nil
}

func environment(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	result := make([]string, 0, len(values))
	for _, key := range keys {
		result = append(result, key+"="+values[key])
	}
	return result
}

func runJSON(ctx context.Context, plan shared.Plan, request any, response any) error {
	payload, err := json.Marshal(request)
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, plan.Argv[0], plan.Argv[1:]...)
	command.Env = append(os.Environ(), environment(plan.Env)...)
	command.Stdin = bytes.NewReader(payload)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	decoder := json.NewDecoder(bytes.NewReader(output))
	if err := decoder.Decode(response); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("provider returned more than one JSON response")
	}
	return nil
}

func Loaded() ([]shared.LoadedManifest, error) {
	var result []shared.LoadedManifest
	seen := map[string]bool{}
	directories, err := directories()
	if err != nil {
		return nil, err
	}
	for _, directory := range directories {
		loaded, err := shared.Discover(directory)
		if err != nil {
			return nil, err
		}
		for _, item := range loaded {
			if !seen[item.Manifest.Name] {
				seen[item.Manifest.Name] = true
				result = append(result, item)
			}
		}
	}
	return result, nil
}

func Validate(name, workingDirectory string) ([]shared.ValidationReport, error) {
	loaded, err := Loaded()
	if err != nil {
		return nil, err
	}
	var reports []shared.ValidationReport
	for _, item := range loaded {
		if name == "" || item.Manifest.Name == name || compatAlias(item.Manifest.Name) == name {
			reports = append(reports, (shared.Validator{}).Validate(item.Manifest, workingDirectory))
		}
	}
	if name != "" && len(reports) == 0 {
		return nil, fmt.Errorf("unknown provider %q", name)
	}
	return reports, nil
}

func Schema() ([]byte, error) { return shared.Schema() }
