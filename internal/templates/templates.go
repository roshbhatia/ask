package templates

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"text/template/parse"

	shared "github.com/roshbhatia/go-utils/config"
	"github.com/roshbhatia/go-utils/paths"
	"go.yaml.in/yaml/v3"
)

const (
	legacyPromptVersion = "ask.prompt/v1"
	promptVersion       = "ask.prompt/v2"
	schemaVersion       = "ask.schema/v1"
	promptSchemaURL     = "https://raw.githubusercontent.com/roshbhatia/ask/main/schema/prompt-template.schema.json"
	templateSchemaURL   = "https://raw.githubusercontent.com/roshbhatia/ask/main/schema/schema-template.schema.json"
	typeString          = "string"
	typeBool            = "bool"
	typeInt             = "int"
	typeNumber          = "number"
	typeJSON            = "json"
)

type Variable struct {
	Name        string `json:"name" yaml:"name" jsonschema:"pattern=^[A-Za-z_][A-Za-z0-9_]*$"`
	Type        string `json:"type,omitempty" yaml:"type,omitempty" jsonschema:"enum=string,enum=bool,enum=int,enum=number,enum=json"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Default     string `json:"default,omitempty" yaml:"default,omitempty"`
	Required    bool   `json:"required,omitempty" yaml:"required,omitempty"`
}

type Prompt struct {
	Version     string     `json:"version" yaml:"version" jsonschema:"enum=ask.prompt/v2"`
	Name        string     `json:"name" yaml:"name" jsonschema:"pattern=^[A-Za-z0-9][A-Za-z0-9._-]*$"`
	Description string     `json:"description,omitempty" yaml:"description,omitempty"`
	Prompt      string     `json:"prompt" yaml:"prompt"`
	Schema      string     `json:"schema,omitempty" yaml:"schema,omitempty" jsonschema:"pattern=^[A-Za-z0-9][A-Za-z0-9._-]*$"`
	Variables   []Variable `json:"variables,omitempty" yaml:"variables,omitempty"`
}

type Schema struct {
	Version     string         `json:"version" yaml:"version" jsonschema:"enum=ask.schema/v1"`
	Name        string         `json:"name" yaml:"name" jsonschema:"pattern=^[A-Za-z0-9][A-Za-z0-9._-]*$"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	Schema      map[string]any `json:"schema" yaml:"schema"`
}

var (
	validName         = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	validVariable     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	legacyVariable    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)
	legacyPlaceholder = regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_.-]*)\s*\}\}`)
)

func Dir() string {
	return filepath.Join(paths.ConfigHome(), "ask", "templates")
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
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(into); err != nil {
		return fmt.Errorf("%s will not parse: %w", path, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("%s contains more than one YAML document", path)
		}
		return fmt.Errorf("%s will not parse: %w", path, err)
	}
	return nil
}

func write(path, schemaURL string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	raw = append([]byte("# yaml-language-server: $schema="+schemaURL+"\n"), raw...)
	temporary, err := os.CreateTemp(filepath.Dir(path), ".template-*.yaml")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer func() { _ = os.Remove(name) }()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(raw); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
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
	if prompt.Version != promptVersion && prompt.Version != legacyPromptVersion {
		return Prompt{}, fmt.Errorf("prompt template %q has version %q, want %q or %q", name, prompt.Version, promptVersion, legacyPromptVersion)
	}
	if prompt.Name != name {
		return Prompt{}, fmt.Errorf("prompt template %q declares name %q", name, prompt.Name)
	}
	if strings.TrimSpace(prompt.Prompt) == "" {
		return Prompt{}, fmt.Errorf("prompt template %q has no prompt", name)
	}
	if err := validatePrompt(prompt); err != nil {
		return Prompt{}, err
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
	if err := validatePrompt(prompt); err != nil {
		return "", err
	}
	if prompt.Schema != "" {
		if _, err := LoadSchema(prompt.Schema); err != nil {
			return "", fmt.Errorf("default schema: %w", err)
		}
	}
	return path, write(path, promptSchemaURL, prompt)
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
	return path, write(path, templateSchemaURL, schema)
}

func validateVariables(variables []Variable) error {
	return validateVariablesForVersion(variables, promptVersion)
}

func validateVariablesForVersion(variables []Variable, version string) error {
	seen := map[string]bool{}
	for _, variable := range variables {
		namePattern := validVariable
		if version == legacyPromptVersion {
			namePattern = legacyVariable
		}
		if !namePattern.MatchString(variable.Name) {
			return fmt.Errorf("invalid variable name %q; use a Go identifier", variable.Name)
		}
		if seen[variable.Name] {
			return fmt.Errorf("variable %q is declared more than once", variable.Name)
		}
		seen[variable.Name] = true
		if _, err := parseValue(variable, sampleValue(variable.Type)); err != nil {
			return fmt.Errorf("variable %q: %w", variable.Name, err)
		}
		if _, err := parseValue(variable, variable.Default); err != nil && !variable.Required {
			return fmt.Errorf("default for variable %q: %w", variable.Name, err)
		}
	}
	return nil
}

func ParseVariables(specs []string) ([]Variable, error) {
	variables := make([]Variable, 0, len(specs))
	for _, spec := range specs {
		declaration, value, hasDefault := strings.Cut(spec, "=")
		name, variableType, hasType := strings.Cut(declaration, ":")
		name = strings.TrimSpace(name)
		variableType = strings.TrimSpace(variableType)
		if !hasType || variableType == "" {
			variableType = typeString
		}
		variable := Variable{Name: name, Type: variableType, Required: !hasDefault}
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
	if prompt.Version == legacyPromptVersion {
		return resolveLegacy(prompt, supplied)
	}
	if err := validateVariablesForVersion(prompt.Variables, prompt.Version); err != nil {
		return "", nil, err
	}
	declared := make(map[string]Variable, len(prompt.Variables))
	values := make(map[string]any, len(prompt.Variables))
	for _, variable := range prompt.Variables {
		declared[variable.Name] = variable
		if variable.Default != "" || !variable.Required {
			value, err := parseValue(variable, variable.Default)
			if err != nil {
				return "", nil, fmt.Errorf("default for variable %q: %w", variable.Name, err)
			}
			values[variable.Name] = value
		}
	}
	for name, raw := range supplied {
		variable, ok := declared[name]
		if !ok {
			return "", nil, fmt.Errorf("prompt template %q has no variable %q", prompt.Name, name)
		}
		value, err := parseValue(variable, raw)
		if err != nil {
			return "", nil, fmt.Errorf("variable %q: %w", name, err)
		}
		values[name] = value
	}

	missing := make([]Variable, 0)
	for _, variable := range prompt.Variables {
		if _, ok := values[variable.Name]; !ok && variable.Required {
			missing = append(missing, variable)
		}
	}
	if len(missing) > 0 {
		return "", missing, nil
	}

	parsed, err := parsePrompt(prompt)
	if err != nil {
		return "", nil, err
	}
	var rendered strings.Builder
	if err := parsed.Execute(&rendered, values); err != nil {
		return "", nil, fmt.Errorf("render prompt template %q: %w", prompt.Name, err)
	}
	return rendered.String(), nil, nil
}

func validatePrompt(prompt Prompt) error {
	if prompt.Version == legacyPromptVersion {
		return validateLegacyPrompt(prompt)
	}
	if err := validateVariablesForVersion(prompt.Variables, prompt.Version); err != nil {
		return err
	}
	parsed, err := parsePrompt(prompt)
	if err != nil {
		return err
	}
	declared := make(map[string]bool, len(prompt.Variables))
	for _, variable := range prompt.Variables {
		declared[variable.Name] = true
	}
	if err := validateTemplateReferences(parsed, declared); err != nil {
		return fmt.Errorf("prompt template %q: %w", prompt.Name, err)
	}
	values := make(map[string]string, len(prompt.Variables))
	for _, variable := range prompt.Variables {
		values[variable.Name] = sampleValue(variable.Type)
	}
	_, missing, err := Resolve(prompt, values)
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		return errors.New("prompt template validation unexpectedly lacked sample values")
	}
	return nil
}

func validateLegacyPrompt(prompt Prompt) error {
	if err := validateVariablesForVersion(prompt.Variables, prompt.Version); err != nil {
		return err
	}
	declared := make(map[string]bool, len(prompt.Variables))
	for _, variable := range prompt.Variables {
		declared[variable.Name] = true
	}
	for _, match := range legacyPlaceholder.FindAllStringSubmatch(prompt.Prompt, -1) {
		if !declared[match[1]] {
			return fmt.Errorf("prompt template %q uses undeclared variable %q", prompt.Name, match[1])
		}
	}
	return nil
}

func resolveLegacy(prompt Prompt, supplied map[string]string) (string, []Variable, error) {
	if err := validateLegacyPrompt(prompt); err != nil {
		return "", nil, err
	}
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
	missing := make([]Variable, 0)
	for _, variable := range prompt.Variables {
		if _, ok := values[variable.Name]; !ok && variable.Required {
			missing = append(missing, variable)
		}
	}
	if len(missing) > 0 {
		return "", missing, nil
	}
	rendered := legacyPlaceholder.ReplaceAllStringFunc(prompt.Prompt, func(found string) string {
		name := legacyPlaceholder.FindStringSubmatch(found)[1]
		return values[name]
	})
	return rendered, nil, nil
}

func parsePrompt(prompt Prompt) (*template.Template, error) {
	parsed, err := template.New(prompt.Name).Option("missingkey=error").Funcs(template.FuncMap{
		"json": func(value any) (string, error) {
			raw, err := json.Marshal(value)
			return string(raw), err
		},
	}).Parse(prompt.Prompt)
	if err != nil {
		return nil, fmt.Errorf("parse prompt template %q: %w", prompt.Name, err)
	}
	return parsed, nil
}

func validateTemplateReferences(parsed *template.Template, declared map[string]bool) error {
	validator := referenceValidator{
		templates:   parsed,
		declared:    declared,
		visited:     map[string]map[bool]bool{},
		rootAliases: map[string]bool{},
	}
	return validator.template(parsed.Name(), true)
}

type referenceValidator struct {
	templates   *template.Template
	declared    map[string]bool
	visited     map[string]map[bool]bool
	rootAliases map[string]bool
}

func (validator *referenceValidator) template(name string, root bool) error {
	seen := validator.visited[name]
	if seen == nil {
		seen = map[bool]bool{}
		validator.visited[name] = seen
	}
	if seen[root] {
		return nil
	}
	seen[root] = true
	defined := validator.templates.Lookup(name)
	if defined == nil || defined.Tree == nil || defined.Tree.Root == nil {
		return nil
	}
	return validator.node(defined.Tree.Root, root)
}

func (validator *referenceValidator) node(node parse.Node, root bool) error {
	if node == nil {
		return nil
	}
	switch current := node.(type) {
	case *parse.ListNode:
		if current == nil {
			return nil
		}
		for _, child := range current.Nodes {
			if err := validator.node(child, root); err != nil {
				return err
			}
		}
	case *parse.ActionNode:
		return validator.node(current.Pipe, root)
	case *parse.PipeNode:
		if current == nil {
			return nil
		}
		for _, command := range current.Cmds {
			if err := validator.node(command, root); err != nil {
				return err
			}
		}
		if validator.pipeProvidesRoot(current, root) {
			for _, declaration := range current.Decl {
				if len(declaration.Ident) > 0 {
					validator.rootAliases[declaration.Ident[0]] = true
				}
			}
		}
	case *parse.CommandNode:
		if name, ok := validator.indexedRootKey(current, root); ok {
			if err := validator.reference(name); err != nil {
				return err
			}
		}
		for _, argument := range current.Args {
			if err := validator.node(argument, root); err != nil {
				return err
			}
		}
	case *parse.FieldNode:
		if root && len(current.Ident) > 0 {
			return validator.reference(current.Ident[0])
		}
	case *parse.VariableNode:
		if len(current.Ident) > 1 && (current.Ident[0] == "$" || validator.rootAliases[current.Ident[0]]) {
			return validator.reference(current.Ident[1])
		}
	case *parse.ChainNode:
		if err := validator.node(current.Node, root); err != nil {
			return err
		}
		if root && len(current.Field) > 0 {
			switch current.Node.(type) {
			case *parse.DotNode:
				return validator.reference(current.Field[0])
			}
		}
	case *parse.IfNode:
		if err := validator.node(current.Pipe, root); err != nil {
			return err
		}
		if err := validator.node(current.List, root); err != nil {
			return err
		}
		return validator.node(current.ElseList, root)
	case *parse.WithNode:
		if err := validator.node(current.Pipe, root); err != nil {
			return err
		}
		if err := validator.node(current.List, validator.pipeProvidesRoot(current.Pipe, root)); err != nil {
			return err
		}
		return validator.node(current.ElseList, root)
	case *parse.RangeNode:
		if err := validator.node(current.Pipe, root); err != nil {
			return err
		}
		if err := validator.node(current.List, false); err != nil {
			return err
		}
		return validator.node(current.ElseList, root)
	case *parse.TemplateNode:
		if err := validator.node(current.Pipe, root); err != nil {
			return err
		}
		return validator.template(current.Name, validator.pipeProvidesRoot(current.Pipe, root))
	}
	return nil
}

func (validator *referenceValidator) indexedRootKey(command *parse.CommandNode, root bool) (string, bool) {
	if len(command.Args) != 3 {
		return "", false
	}
	identifier, ok := command.Args[0].(*parse.IdentifierNode)
	if !ok || identifier.Ident != "index" {
		return "", false
	}
	rootValue := root
	if variable, ok := command.Args[1].(*parse.VariableNode); ok {
		rootValue = len(variable.Ident) == 1 && (variable.Ident[0] == "$" || validator.rootAliases[variable.Ident[0]])
	} else if _, ok := command.Args[1].(*parse.DotNode); !ok {
		return "", false
	}
	key, ok := command.Args[2].(*parse.StringNode)
	return key.Text, ok && rootValue
}

func (validator *referenceValidator) reference(name string) error {
	if !validator.declared[name] {
		return fmt.Errorf("uses undeclared variable %q", name)
	}
	return nil
}

func (validator *referenceValidator) pipeProvidesRoot(pipe *parse.PipeNode, dotIsRoot bool) bool {
	if pipe == nil || len(pipe.Cmds) != 1 || len(pipe.Cmds[0].Args) != 1 {
		return false
	}
	switch argument := pipe.Cmds[0].Args[0].(type) {
	case *parse.DotNode:
		return dotIsRoot
	case *parse.VariableNode:
		return len(argument.Ident) == 1 && (argument.Ident[0] == "$" || validator.rootAliases[argument.Ident[0]])
	default:
		return false
	}
}

func sampleValue(variableType string) string {
	switch normalizedType(variableType) {
	case typeBool:
		return "false"
	case typeInt, typeNumber:
		return "0"
	case typeJSON:
		return "null"
	default:
		return ""
	}
}

func normalizedType(variableType string) string {
	if variableType == "" {
		return typeString
	}
	return variableType
}

func parseValue(variable Variable, raw string) (any, error) {
	switch normalizedType(variable.Type) {
	case typeString:
		return raw, nil
	case typeBool:
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("want bool, got %q", raw)
		}
		return value, nil
	case typeInt:
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("want int, got %q", raw)
		}
		return value, nil
	case typeNumber:
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
			return nil, fmt.Errorf("want finite number, got %q", raw)
		}
		return value, nil
	case typeJSON:
		var value any
		decoder := json.NewDecoder(strings.NewReader(raw))
		decoder.UseNumber()
		if err := decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("want JSON, got %q: %w", raw, err)
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("want one JSON value, got %q", raw)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unknown type %q; use string, bool, int, number, or json", variable.Type)
	}
}

func PromptSchema() ([]byte, error) {
	return shared.Schema[Prompt]("Ask prompt template")
}

func SchemaTemplateSchema() ([]byte, error) {
	return shared.Schema[Schema]("Ask schema template")
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
