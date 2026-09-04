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
	"path/filepath"
	"slices"
	"strings"
	"time"

	shared "github.com/roshbhatia/go-utils/provider"

	"github.com/roshbhatia/ask/internal/process"
)

const (
	ActionGenerate = "inference.generate"
	ActionModels   = "inference.models"
	ActionValidate = "provider.validate"
	Protocol       = "provider/v1"
)

type Request struct {
	Prompt string         `json:"prompt"`
	Input  string         `json:"input,omitempty"`
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
	Version string  `json:"version,omitempty" jsonschema:"enum=provider/v1"`
	Kind    Kind    `json:"type" jsonschema:"enum=started,enum=text,enum=tool,enum=notice,enum=result"`
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
	Version string  `json:"version" jsonschema:"enum=provider/v1"`
	Action  string  `json:"action" jsonschema:"enum=inference.generate,enum=inference.models,enum=provider.validate"`
	Request Request `json:"request"`
}

type ValidationResponse struct {
	Version string `json:"version" jsonschema:"enum=provider/v1"`
	Status  string `json:"status" jsonschema:"enum=ok"`
}

type ModelResponse struct {
	Version string   `json:"version" jsonschema:"enum=provider/v1"`
	Models  []string `json:"models"`
}

type Provider interface {
	Name() string
	Run(context.Context, Request) (<-chan Event, error)
}

type Info struct {
	Name   string
	Blurb  string
	Binary string

	manifest shared.Manifest
	path     string
}

func (i Info) New() Provider { return commandProvider{manifest: i.manifest, path: i.path} }

func (i Info) Ready() bool {
	return (shared.Validator{}).Validate(i.manifest, filepath.Dir(i.path)).OK()
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
	plan = resolvePlanCommand(plan, i.path)
	value := ModelResponse{}
	if err := runJSON(ctx, plan, Envelope{Version: Protocol, Action: ActionModels}, &value, ""); err != nil || value.Version != Protocol {
		return nil
	}
	return slices.Clone(value.Models)
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
	dataHome := strings.TrimSpace(os.Getenv("XDG_DATA_HOME"))
	if dataHome == "" {
		if home, err := os.UserHomeDir(); err == nil {
			dataHome = filepath.Join(home, ".local", "share")
		}
	}
	if dataHome != "" {
		result = append(result, filepath.Join(dataHome, "ask", "providers"))
	}
	if executable, err := os.Executable(); err == nil {
		binaryDirectory := filepath.Dir(executable)
		result = append(result,
			filepath.Join(binaryDirectory, "providers"),
			filepath.Join(binaryDirectory, "..", "share", "ask", "providers"),
		)
	}
	dataDirectories := strings.TrimSpace(os.Getenv("XDG_DATA_DIRS"))
	if dataDirectories == "" {
		dataDirectories = "/usr/local/share:/usr/share"
	}
	for _, directory := range filepath.SplitList(dataDirectories) {
		if directory != "" {
			result = append(result, filepath.Join(directory, "ask", "providers"))
		}
	}
	return uniquePaths(result)
}

func uniquePaths(paths []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		path = filepath.Clean(path)
		if path == "." || seen[path] {
			continue
		}
		seen[path] = true
		result = append(result, path)
	}
	return result
}

type manifestDiagnostic struct {
	path string
	err  error
}

func directories() ([]string, []manifestDiagnostic) {
	var result []string
	var diagnostics []manifestDiagnostic
	for _, root := range providerRoots() {
		entries, err := os.ReadDir(root)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			diagnostics = append(diagnostics, manifestDiagnostic{path: root, err: err})
			continue
		}
		result = append(result, root)
		for _, entry := range entries {
			path := filepath.Join(root, entry.Name())
			isDirectory := entry.IsDir()
			if entry.Type()&os.ModeSymlink != 0 {
				info, statErr := os.Stat(path)
				if statErr != nil {
					diagnostics = append(diagnostics, manifestDiagnostic{path: path, err: statErr})
					continue
				}
				isDirectory = info.IsDir()
			}
			if isDirectory {
				result = append(result, path)
			}
		}
	}
	return result, diagnostics
}

func loadManifests() ([]shared.LoadedManifest, []manifestDiagnostic, error) {
	directories, diagnostics := directories()
	var loaded []shared.LoadedManifest
	for _, directory := range directories {
		entries, err := os.ReadDir(directory)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			diagnostics = append(diagnostics, manifestDiagnostic{path: directory, err: err})
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			extension := filepath.Ext(entry.Name())
			if extension != ".json" && extension != ".yaml" && extension != ".yml" {
				continue
			}
			path := filepath.Join(directory, entry.Name())
			raw, err := os.ReadFile(path)
			if err != nil {
				diagnostics = append(diagnostics, manifestDiagnostic{path: path, err: err})
				continue
			}
			manifest, err := shared.Decode(bytes.NewReader(raw), extension)
			if err != nil {
				diagnostics = append(diagnostics, manifestDiagnostic{path: path, err: err})
				continue
			}
			loaded = append(loaded, shared.LoadedManifest{Manifest: manifest, Path: path})
		}
	}
	return dedupeLoaded(loaded), diagnostics, nil
}

func dedupeLoaded(loaded []shared.LoadedManifest) []shared.LoadedManifest {
	seen := make(map[string]bool, len(loaded))
	result := make([]shared.LoadedManifest, 0, len(loaded))
	for _, item := range loaded {
		if seen[item.Manifest.Name] {
			continue
		}
		seen[item.Manifest.Name] = true
		result = append(result, item)
	}
	return result
}

func Discover() ([]Info, error) {
	var result []Info
	loaded, _, err := loadManifests()
	if err != nil {
		return nil, err
	}
	for _, item := range loaded {
		manifest := item.Manifest
		if _, ok := manifest.Actions[ActionGenerate]; !ok {
			continue
		}
		result = append(result, Info{
			Name: manifest.Name, Blurb: manifest.Description, Binary: manifest.Command[0],
			manifest: manifest, path: item.Path,
		})
	}
	slices.SortFunc(result, func(a, b Info) int { return strings.Compare(a.Name, b.Name) })
	return result, nil
}

func Known() ([]Info, error) {
	return Discover()
}

func Lookup(name string) (Info, bool, error) {
	providers, err := Discover()
	if err != nil {
		return Info{}, false, err
	}
	for _, one := range providers {
		if name == one.Name {
			return one, true, nil
		}
	}
	return Info{}, false, nil
}

func Names() ([]string, error) {
	providers, err := Discover()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(providers))
	for _, one := range providers {
		names = append(names, one.Name)
	}
	return names, nil
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
		if name == one.Name {
			return one.New(), nil
		}
	}
	known := make([]string, 0, len(providers))
	for _, one := range providers {
		known = append(known, one.Name)
	}
	if len(known) == 0 {
		return nil, fmt.Errorf("provider %q is not installed", name)
	}
	return nil, fmt.Errorf("unknown provider %q, known: %s", name, strings.Join(known, ", "))
}

type commandProvider struct {
	manifest shared.Manifest
	path     string
}

func (p commandProvider) Name() string { return p.manifest.Name }

func (p commandProvider) Run(ctx context.Context, request Request) (<-chan Event, error) {
	plan, err := p.manifest.Render(ActionGenerate, request)
	if err != nil {
		return nil, err
	}
	plan = resolvePlanCommand(plan, p.path)
	cancel := func() {}
	if plan.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, plan.Timeout)
	}
	payload, err := json.Marshal(Envelope{Version: Protocol, Action: ActionGenerate, Request: request})
	if err != nil {
		cancel()
		return nil, err
	}
	command := process.CommandContext(ctx, plan.Argv[0], plan.Argv[1:]...)
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
		var terminal *Event
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
			if terminal != nil {
				protocolError = "provider returned an event after its result"
				continue
			}
			if event.Kind == Done {
				copy := event
				terminal = &copy
				continue
			}
			if !sendEvent(ctx, events, event) {
				return
			}
		}
		waitErr := command.Wait()
		if scanErr := scanner.Err(); protocolError == "" && scanErr != nil {
			protocolError = scanErr.Error()
		}
		if protocolError == "" && waitErr != nil {
			protocolError = strings.TrimSpace(stderr.String())
			if protocolError == "" {
				protocolError = waitErr.Error()
			}
		}
		if protocolError != "" {
			sendEvent(ctx, events, Event{Version: Protocol, Kind: Done, Result: &Result{Failed: true, Reason: protocolError}})
			return
		}
		if terminal != nil {
			sendEvent(ctx, events, *terminal)
			return
		}
		reason := strings.TrimSpace(stderr.String())
		if reason == "" {
			reason = p.manifest.Name + " exited without an answer"
		}
		sendEvent(ctx, events, Event{Version: Protocol, Kind: Done, Result: &Result{Failed: true, Reason: reason}})
	}()
	return events, nil
}

func sendEvent(ctx context.Context, events chan<- Event, event Event) bool {
	select {
	case events <- event:
		return true
	default:
	}
	select {
	case events <- event:
		return true
	case <-ctx.Done():
		return false
	}
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

func runJSON(ctx context.Context, plan shared.Plan, request any, response any, directory string) error {
	payload, err := json.Marshal(request)
	if err != nil {
		return err
	}
	command := process.CommandContext(ctx, plan.Argv[0], plan.Argv[1:]...)
	command.Dir = directory
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

func resolvePlanCommand(plan shared.Plan, manifestPath string) shared.Plan {
	if len(plan.Argv) == 0 || filepath.IsAbs(plan.Argv[0]) || !strings.ContainsRune(plan.Argv[0], filepath.Separator) {
		return plan
	}
	plan.Argv = slices.Clone(plan.Argv)
	plan.Argv[0] = filepath.Clean(filepath.Join(filepath.Dir(manifestPath), plan.Argv[0]))
	return plan
}

func Loaded() ([]shared.LoadedManifest, error) {
	loaded, _, err := loadManifests()
	if err != nil {
		return nil, err
	}
	return loaded, nil
}

func Validate(name, workingDirectory string) ([]shared.ValidationReport, error) {
	loaded, diagnostics, err := loadedForValidation(name)
	if err != nil {
		return nil, err
	}
	var reports []shared.ValidationReport
	for _, item := range loaded {
		if name == "" || item.Manifest.Name == name {
			report := (shared.Validator{}).Validate(item.Manifest, filepath.Dir(item.Path))
			validateContract(&report, item.Manifest, item.Path, workingDirectory)
			reports = append(reports, report)
		}
	}
	for _, diagnostic := range diagnostics {
		reports = append(reports, shared.ValidationReport{
			Provider: diagnosticName(diagnostic.path),
			Checks: []shared.Check{{
				Kind: "manifest", Target: diagnostic.path, Status: shared.CheckFailed, Message: diagnostic.err.Error(),
			}},
		})
	}
	if name != "" && len(reports) == 0 {
		return nil, fmt.Errorf("unknown provider %q", name)
	}
	return reports, nil
}

func diagnosticName(path string) string {
	parent := filepath.Base(filepath.Dir(path))
	if parent != "providers" {
		return parent
	}
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

func loadedForValidation(name string) ([]shared.LoadedManifest, []manifestDiagnostic, error) {
	if name == "" {
		return loadManifests()
	}
	if filepath.Base(name) != name || name == "." || name == ".." {
		return nil, nil, fmt.Errorf("invalid provider name %q", name)
	}
	loaded, diagnostics, err := loadManifests()
	if err != nil {
		return nil, nil, err
	}
	for _, item := range loaded {
		if item.Manifest.Name == name {
			return []shared.LoadedManifest{item}, nil, nil
		}
	}
	for _, diagnostic := range diagnostics {
		if diagnosticName(diagnostic.path) == name {
			return nil, nil, fmt.Errorf("decode provider manifest %s: %w", diagnostic.path, diagnostic.err)
		}
	}
	return nil, nil, nil
}

func validateContract(report *shared.ValidationReport, manifest shared.Manifest, manifestPath, workingDirectory string) {
	for _, action := range []string{ActionGenerate, ActionValidate} {
		status := shared.CheckOK
		message := "declared"
		if _, ok := manifest.Actions[action]; !ok {
			status = shared.CheckFailed
			message = "required action is not declared"
		}
		report.Checks = append(report.Checks, shared.Check{
			Kind: "action", Target: action, Status: status, Message: message,
		})
	}
	actions := make([]string, 0, len(manifest.Actions))
	for action := range manifest.Actions {
		actions = append(actions, action)
	}
	slices.Sort(actions)
	probe := Request{
		Prompt: "provider validation",
		Input:  "validation input",
		Model:  "validation-model",
		Schema: map[string]any{"type": "object"},
		Dir:    workingDirectory,
	}
	for _, action := range actions {
		status := shared.CheckOK
		message := "rendered"
		if _, err := manifest.Render(action, probe); err != nil {
			status = shared.CheckFailed
			message = err.Error()
		}
		report.Checks = append(report.Checks, shared.Check{
			Kind: "render", Target: action, Status: status, Message: message,
		})
	}
	if !report.OK() {
		return
	}
	plan, err := manifest.Render(ActionValidate, probe)
	if err != nil {
		report.Checks = append(report.Checks, contractCheck(shared.CheckFailed, err.Error()))
		return
	}
	plan = resolvePlanCommand(plan, manifestPath)
	timeout := plan.Timeout
	if timeout <= 0 || timeout > 5*time.Second {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	response := ValidationResponse{}
	err = runJSON(ctx, plan, Envelope{Version: Protocol, Action: ActionValidate, Request: probe}, &response, workingDirectory)
	if err != nil {
		report.Checks = append(report.Checks, contractCheck(shared.CheckFailed, err.Error()))
		return
	}
	if response.Version != Protocol || response.Status != "ok" {
		message := fmt.Sprintf("got version %q and status %q", response.Version, response.Status)
		report.Checks = append(report.Checks, contractCheck(shared.CheckFailed, message))
		return
	}
	report.Checks = append(report.Checks, contractCheck(shared.CheckOK, "adapter probe passed"))
}

func contractCheck(status shared.CheckStatus, message string) shared.Check {
	return shared.Check{Kind: "contract", Target: ActionValidate, Status: status, Message: message}
}
