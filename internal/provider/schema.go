package provider

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/invopop/jsonschema"
	shared "github.com/roshbhatia/go-utils/provider"
)

func Schema() ([]byte, error) {
	raw, err := shared.Schema()
	if err != nil {
		return nil, err
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, err
	}

	properties, err := schemaObject(schema, "properties")
	if err != nil {
		return nil, err
	}
	actions, err := schemaObject(properties, "actions")
	if err != nil {
		return nil, err
	}
	actions["minProperties"] = 2
	actions["required"] = []string{ActionGenerate, ActionValidate}
	actions["propertyNames"] = map[string]any{"pattern": `^[a-z][a-z0-9._-]*$`}

	description, err := schemaObject(properties, "description")
	if err != nil {
		return nil, err
	}
	description["minLength"] = 1
	command, err := schemaObject(properties, "command")
	if err != nil {
		return nil, err
	}
	command["items"] = nonEmptyStringSchema()

	definitions, err := schemaObject(schema, "$defs")
	if err != nil {
		return nil, err
	}
	if err := constrainActions(definitions); err != nil {
		return nil, err
	}
	if err := constrainRequirements(definitions); err != nil {
		return nil, err
	}
	return encodeSchema(schema)
}

func constrainActions(definitions map[string]any) error {
	action, err := schemaObject(definitions, "Action")
	if err != nil {
		return err
	}
	properties, err := schemaObject(action, "properties")
	if err != nil {
		return err
	}
	description, err := schemaObject(properties, "description")
	if err != nil {
		return err
	}
	description["minLength"] = 1
	environment, err := schemaObject(properties, "env")
	if err != nil {
		return err
	}
	environment["propertyNames"] = environmentNameSchema()
	return nil
}

func constrainRequirements(definitions map[string]any) error {
	requirements, err := schemaObject(definitions, "Requirements")
	if err != nil {
		return err
	}
	properties, err := schemaObject(requirements, "properties")
	if err != nil {
		return err
	}
	commands, err := schemaObject(properties, "commands")
	if err != nil {
		return err
	}
	commands["items"] = nonEmptyStringSchema()
	environment, err := schemaObject(properties, "environment")
	if err != nil {
		return err
	}
	environment["items"] = environmentNameSchema()
	paths, err := schemaObject(properties, "paths")
	if err != nil {
		return err
	}
	paths["items"] = nonEmptyStringSchema()
	return nil
}

func schemaObject(parent map[string]any, key string) (map[string]any, error) {
	value, ok := parent[key].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("provider schema field %q is not an object", key)
	}
	return value, nil
}

func nonEmptyStringSchema() map[string]any {
	return map[string]any{"type": "string", "minLength": 1}
}

func environmentNameSchema() map[string]any {
	return map[string]any{
		"type":    "string",
		"pattern": `^[A-Za-z_][A-Za-z0-9_]*$`,
	}
}

func WireSchemas() (map[string][]byte, error) {
	values := map[string]any{
		"protocol.event.schema.json":      new(Event),
		"protocol.models.schema.json":     new(ModelResponse),
		"protocol.request.schema.json":    new(Envelope),
		"protocol.validation.schema.json": new(ValidationResponse),
	}
	result := make(map[string][]byte, len(values))
	for name, value := range values {
		reflector := jsonschema.Reflector{Anonymous: true, ExpandedStruct: true}
		schema := reflector.Reflect(value)
		title := strings.TrimSuffix(strings.TrimPrefix(name, "protocol."), ".schema.json")
		schema.Title = "Ask provider " + title
		encoded, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			return nil, err
		}
		result[name] = append(encoded, '\n')
	}
	return result, nil
}

func encodeSchema(schema map[string]any) ([]byte, error) {
	encoded, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}
