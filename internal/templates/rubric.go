package templates

import (
	"fmt"
	"path/filepath"

	"github.com/roshbhatia/ask/internal/evaluation"
	shared "github.com/roshbhatia/go-utils/config"
)

type Rubric struct {
	Version     string               `json:"version" yaml:"version" jsonschema:"enum=ask.rubric/v1"`
	Name        string               `json:"name" yaml:"name" jsonschema:"pattern=^[A-Za-z0-9][A-Za-z0-9._-]*$"`
	Description string               `json:"description,omitempty" yaml:"description,omitempty"`
	Questions   evaluation.Questions `json:"questions" yaml:"questions"`
}

func RubricDir() string { return filepath.Join(Dir(), "rubrics") }

func LoadRubric(name string) (Rubric, error) {
	path, err := templatePath(RubricDir(), name)
	if err != nil {
		return Rubric{}, err
	}
	var rubric Rubric
	if err := read(path, &rubric); err != nil {
		return rubric, err
	}
	if rubric.Version != "ask.rubric/v1" || rubric.Name != name {
		return rubric, fmt.Errorf("invalid version or name in rubric %q", name)
	}
	return rubric, rubric.Questions.Validate()
}

func SaveRubric(rubric Rubric) error {
	path, err := templatePath(RubricDir(), rubric.Name)
	if err != nil {
		return err
	}
	if err := rubric.Questions.Validate(); err != nil {
		return err
	}
	rubric.Version = "ask.rubric/v1"
	return write(path, "https://raw.githubusercontent.com/roshbhatia/ask/main/schema/rubric-template.schema.json", rubric)
}

func RubricSchema() ([]byte, error) { return shared.Schema[Rubric]("Ask evaluation rubric") }
