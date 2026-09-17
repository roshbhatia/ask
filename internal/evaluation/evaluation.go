package evaluation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
	"unicode/utf8"
)

type Question struct {
	Type         string            `json:"type" yaml:"type" jsonschema:"enum=boolean,enum=choice,enum=score"`
	Instructions string            `json:"instructions" yaml:"instructions"`
	Choices      map[string]string `json:"choices,omitempty" yaml:"choices,omitempty"`
	Levels       []string          `json:"levels,omitempty" yaml:"levels,omitempty"`
}

type Questions map[string]Question

type Answer struct {
	Value        any                `json:"value"`
	Probability  *float64           `json:"probability,omitempty"`
	Distribution map[string]float64 `json:"distribution,omitempty"`
}

type Result struct {
	Version   string            `json:"version" jsonschema:"enum=ask.evaluation/v1"`
	Provider  string            `json:"provider"`
	Model     string            `json:"model,omitempty"`
	Method    string            `json:"method" jsonschema:"enum=native,enum=structured"`
	Questions Questions         `json:"questions"`
	Answers   map[string]Answer `json:"answers"`
	Metadata  map[string]any    `json:"metadata,omitempty"`
}

func (questions Questions) Validate() error {
	if len(questions) == 0 {
		return errors.New("at least one evaluation question is required")
	}
	for id, q := range questions {
		if strings.TrimSpace(id) == "" || strings.TrimSpace(q.Instructions) == "" {
			return fmt.Errorf("question %q needs an ID and instructions", id)
		}
		switch q.Type {
		case "boolean":
			if len(q.Choices) != 0 || len(q.Levels) != 0 {
				return fmt.Errorf("boolean %q cannot have choices or levels", id)
			}
		case "choice":
			if len(q.Choices) < 2 || len(q.Levels) != 0 {
				return fmt.Errorf("choice %q needs at least two choices and no levels", id)
			}
			for name, description := range q.Choices {
				if strings.TrimSpace(name) == "" || strings.TrimSpace(description) == "" {
					return fmt.Errorf("choice %q has an empty label or description", id)
				}
			}
		case "score":
			if len(q.Levels) < 2 || len(q.Choices) != 0 {
				return fmt.Errorf("score %q needs at least two ordered levels and no choices", id)
			}
			seen := map[string]bool{}
			for _, level := range q.Levels {
				if strings.TrimSpace(level) == "" || seen[level] {
					return fmt.Errorf("score %q has empty or duplicate levels", id)
				}
				seen[level] = true
			}
		default:
			return fmt.Errorf("question %q has unknown type %q", id, q.Type)
		}
	}
	return nil
}

func Decode(raw []byte, target any) error {
	if err := uniqueKeys(raw); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("expected exactly one JSON document")
	}
	return nil
}

func uniqueKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var walk func() error
	walk = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, compound := token.(json.Delim)
		if !compound {
			return nil
		}
		seen := map[string]bool{}
		for decoder.More() {
			if delimiter == '{' {
				key, err := decoder.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok {
					return errors.New("JSON object key is not a string")
				}
				if seen[name] {
					return fmt.Errorf("duplicate JSON key %q", name)
				}
				seen[name] = true
			}
			if err := walk(); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	}
	if err := walk(); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("expected exactly one JSON document")
	}
	return nil
}

func State(raw []byte, format string) (any, error) {
	if !utf8.Valid(raw) || bytes.ContainsRune(raw, 0) {
		return nil, errors.New("evaluation input must be UTF-8 text, not binary data")
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, errors.New("evaluation input is empty")
	}
	if format == "text" {
		return string(raw), nil
	}
	if format != "auto" && format != "json" {
		return nil, fmt.Errorf("unknown input format %q", format)
	}
	var state any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	err := decoder.Decode(&state)
	if err == nil && decoder.Decode(new(any)) != io.EOF {
		err = errors.New("expected exactly one JSON value")
	}
	if err == nil {
		return state, uniqueKeys(raw)
	}
	if format == "json" || strings.ContainsRune("{[\"", rune(bytes.TrimSpace(raw)[0])) {
		return nil, fmt.Errorf("invalid JSON input; use --input text for literal text: %w", err)
	}
	return string(raw), nil
}

func (questions Questions) Schema() map[string]any {
	properties := map[string]any{}
	ids := make([]string, 0, len(questions))
	for id, question := range questions {
		ids = append(ids, id)
		field := map[string]any{"description": question.Instructions}
		switch question.Type {
		case "boolean":
			field["type"] = "boolean"
		case "choice":
			choices := make([]string, 0, len(question.Choices))
			for name := range question.Choices {
				choices = append(choices, name)
			}
			slices.Sort(choices)
			field["type"], field["enum"] = "string", choices
		case "score":
			field["type"], field["minimum"], field["maximum"] = "number", 0, len(question.Levels)-1
		}
		properties[id] = field
	}
	slices.Sort(ids)
	return map[string]any{"type": "object", "properties": properties, "required": ids, "additionalProperties": false}
}

func (questions Questions) Check(answers map[string]Answer, native bool) error {
	if len(answers) != len(questions) {
		return errors.New("evaluation must answer every question exactly once")
	}
	for id, q := range questions {
		a, ok := answers[id]
		if !ok {
			return fmt.Errorf("missing answer %q", id)
		}
		valid := false
		switch q.Type {
		case "boolean":
			_, valid = a.Value.(bool)
		case "choice":
			value, ok := a.Value.(string)
			_, exists := q.Choices[value]
			valid = ok && exists
		case "score":
			value, ok := a.Value.(float64)
			valid = ok && !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= float64(len(q.Levels)-1)
		}
		if !valid {
			return fmt.Errorf("invalid %s answer for %q", q.Type, id)
		}
		if !native && (a.Probability != nil || a.Distribution != nil) {
			return fmt.Errorf("structured answer %q cannot claim native probabilities", id)
		}
		if a.Probability != nil {
			if q.Type != "boolean" || !probability(*a.Probability) {
				return fmt.Errorf("invalid probability for %q", id)
			}
			if a.Value != (*a.Probability >= 0.5) {
				return fmt.Errorf("boolean value disagrees with probability for %q", id)
			}
		}
		if a.Distribution != nil {
			labels := q.Choices
			if q.Type == "score" {
				labels = map[string]string{}
				for _, level := range q.Levels {
					labels[level] = level
				}
			}
			if q.Type == "boolean" || len(labels) != len(a.Distribution) {
				return fmt.Errorf("invalid distribution for %q", id)
			}
			sum := 0.0
			for label, p := range a.Distribution {
				if _, ok := labels[label]; !ok || !probability(p) {
					return fmt.Errorf("invalid distribution for %q", id)
				}
				sum += p
			}
			if math.Abs(sum-1) > 0.000001 {
				return fmt.Errorf("distribution for %q must sum to one", id)
			}
		}
	}
	return nil
}

func probability(value float64) bool { return !math.IsNaN(value) && value >= 0 && value <= 1 }
