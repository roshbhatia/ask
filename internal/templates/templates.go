package templates

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	promptVersion = "ask.prompt/v1"
	schemaVersion = "ask.schema/v1"
)

type Variable struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	Default     string `yaml:"default,omitempty"`
	Required    bool   `yaml:"required,omitempty"`
}

type Prompt struct {
	Version     string     `yaml:"version"`
	Name        string     `yaml:"name"`
	Description string     `yaml:"description,omitempty"`
	Prompt      string     `yaml:"prompt"`
	Schema      string     `yaml:"schema,omitempty"`
	Variables   []Variable `yaml:"variables,omitempty"`
}

type Schema struct {
	Version     string         `yaml:"version"`
	Name        string         `yaml:"name"`
	Description string         `yaml:"description,omitempty"`
	Schema      map[string]any `yaml:"schema"`
}

var (
	validName     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	validVariable = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)
	placeholder   = regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_.-]*)\s*\}\}`)
)

func Dir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "ask", "templates")
}

func PromptDir() string { return filepath.Join(Dir(), "prompts") }

func SchemaDir() string { return filepath.Join(Dir(), "schemas") }

func checkName(name string) error {
	if !validName.MatchString(name) {
		return fmt.Errorf("invalid template name %q; use letters, numbers, dots, dashes, or underscores", name)
	}
	return nil
}

func templatePath(dir, name string) (string, error) {
	if err := checkName(name); err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".yaml"), nil
}

func read(path string, into any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(raw, into); err != nil {
		return fmt.Errorf("%s will not parse: %w", path, err)
	}
	return nil
}

func write(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

func LoadPrompt(name string) (Prompt, error) {
	path, err := templatePath(PromptDir(), name)
	if err != nil {
		return Prompt{}, err
	}
	var prompt Prompt
	if err := read(path, &prompt); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Prompt{}, fmt.Errorf("prompt template %q does not exist", name)
		}
		return Prompt{}, err
	}
	if prompt.Version != promptVersion {
		return Prompt{}, fmt.Errorf("prompt template %q has version %q, want %q", name, prompt.Version, promptVersion)
	}
	if prompt.Name != name {
		return Prompt{}, fmt.Errorf("prompt template %q declares name %q", name, prompt.Name)
	}
	if strings.TrimSpace(prompt.Prompt) == "" {
		return Prompt{}, fmt.Errorf("prompt template %q has no prompt", name)
	}
	return prompt, nil
}

func LoadSchema(name string) (Schema, error) {
	path, err := templatePath(SchemaDir(), name)
	if err != nil {
		return Schema{}, err
	}
	var schema Schema
	if err := read(path, &schema); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Schema{}, fmt.Errorf("schema template %q does not exist", name)
		}
		return Schema{}, err
	}
	if schema.Version != schemaVersion {
		return Schema{}, fmt.Errorf("schema template %q has version %q, want %q", name, schema.Version, schemaVersion)
	}
	if schema.Name != name {
		return Schema{}, fmt.Errorf("schema template %q declares name %q", name, schema.Name)
	}
	if len(schema.Schema) == 0 {
		return Schema{}, fmt.Errorf("schema template %q has no schema", name)
	}
	return schema, nil
}

func SavePrompt(prompt Prompt) (string, error) {
	if strings.TrimSpace(prompt.Prompt) == "" {
		return "", errors.New("cannot save an empty prompt")
	}
	path, err := templatePath(PromptDir(), prompt.Name)
	if err != nil {
		return "", err
	}
	prompt.Version = promptVersion
	if err := validateVariables(prompt.Variables); err != nil {
		return "", err
	}
	values := make(map[string]string, len(prompt.Variables))
	for _, variable := range prompt.Variables {
		values[variable.Name] = ""
	}
	if _, _, err := Resolve(prompt, values); err != nil {
		return "", err
	}
	if prompt.Schema != "" {
		if _, err := LoadSchema(prompt.Schema); err != nil {
			return "", fmt.Errorf("default schema: %w", err)
		}
	}
	return path, write(path, prompt)
}

func SaveSchema(schema Schema) (string, error) {
	if len(schema.Schema) == 0 {
		return "", errors.New("cannot save an empty schema")
	}
	path, err := templatePath(SchemaDir(), schema.Name)
	if err != nil {
		return "", err
	}
	schema.Version = schemaVersion
	return path, write(path, schema)
}

func validateVariables(variables []Variable) error {
	seen := map[string]bool{}
	for _, variable := range variables {
		if !validVariable.MatchString(variable.Name) {
			return fmt.Errorf("invalid variable name %q", variable.Name)
		}
		if seen[variable.Name] {
			return fmt.Errorf("variable %q is declared more than once", variable.Name)
		}
		seen[variable.Name] = true
	}
	return nil
}

func ParseVariables(specs []string) ([]Variable, error) {
	variables := make([]Variable, 0, len(specs))
	for _, spec := range specs {
		name, value, hasDefault := strings.Cut(spec, "=")
		name = strings.TrimSpace(name)
		variable := Variable{Name: name, Required: !hasDefault}
		if hasDefault {
			variable.Default = value
		}
		variables = append(variables, variable)
	}
	if err := validateVariables(variables); err != nil {
		return nil, err
	}
	return variables, nil
}

func Values(pairs []string) (map[string]string, error) {
	values := make(map[string]string, len(pairs))
	for _, pair := range pairs {
		name, value, ok := strings.Cut(pair, "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			return nil, fmt.Errorf("variable %q must use NAME=VALUE", pair)
		}
		values[name] = value
	}
	return values, nil
}

func Resolve(prompt Prompt, supplied map[string]string) (string, []Variable, error) {
	declared := make(map[string]Variable, len(prompt.Variables))
	values := make(map[string]string, len(prompt.Variables))
	for _, variable := range prompt.Variables {
		declared[variable.Name] = variable
		if variable.Default != "" || !variable.Required {
			values[variable.Name] = variable.Default
		}
	}
	for name, value := range supplied {
		if _, ok := declared[name]; !ok {
			return "", nil, fmt.Errorf("prompt template %q has no variable %q", prompt.Name, name)
		}
		values[name] = value
	}

	used := map[string]bool{}
	for _, match := range placeholder.FindAllStringSubmatch(prompt.Prompt, -1) {
		used[match[1]] = true
		if _, ok := declared[match[1]]; !ok {
			return "", nil, fmt.Errorf("prompt template %q uses undeclared variable %q", prompt.Name, match[1])
		}
	}

	missing := make([]Variable, 0)
	for _, variable := range prompt.Variables {
		if _, ok := values[variable.Name]; !ok && (variable.Required || used[variable.Name]) {
			missing = append(missing, variable)
		}
	}
	if len(missing) > 0 {
		return "", missing, nil
	}

	rendered := placeholder.ReplaceAllStringFunc(prompt.Prompt, func(found string) string {
		parts := placeholder.FindStringSubmatch(found)
		return values[parts[1]]
	})
	return rendered, nil, nil
}

func List(kind string) ([]string, error) {
	dir := PromptDir()
	if kind == "schema" {
		dir = SchemaDir()
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		names = append(names, strings.TrimSuffix(entry.Name(), ".yaml"))
	}
	sort.Strings(names)
	return names, nil
}

func Encode(value any) ([]byte, error) { return yaml.Marshal(value) }
