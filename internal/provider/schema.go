package provider

import (
	"encoding/json"
	"strings"

	"github.com/invopop/jsonschema"
	shared "github.com/roshbhatia/go-utils/provider"
)

// Schema is the provider/v1 manifest schema, byte for byte as provider-spec
// publishes it. Ask's own rule, that a provider declares both inference.generate
// and provider.validate, lives in schema/narrow.cue and validateContract.
func Schema() ([]byte, error) {
	return shared.Schema()
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
