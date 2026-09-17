package provider

import (
	"context"
	"fmt"

	"github.com/roshbhatia/ask/internal/evaluation"
)

const ActionEvaluate = "inference.evaluate"

type Capabilities struct {
	Name                 string `json:"name"`
	Generation           bool   `json:"generation"`
	StructuredEvaluation bool   `json:"structuredEvaluation"`
	NativeEvaluation     bool   `json:"nativeEvaluation"`
}

func (i Info) Capabilities() Capabilities {
	return Capabilities{Name: i.Name, Generation: i.Supports(ActionGenerate), StructuredEvaluation: i.Supports(ActionGenerate), NativeEvaluation: i.Supports(ActionEvaluate)}
}

type EvaluationRequest struct {
	State     any                  `json:"state"`
	Questions evaluation.Questions `json:"questions"`
	Model     string               `json:"model,omitempty"`
	Dir       string               `json:"directory"`
}

type EvaluationEnvelope struct {
	Version string            `json:"version" jsonschema:"enum=provider/v1"`
	Action  string            `json:"action" jsonschema:"enum=inference.evaluate"`
	Request EvaluationRequest `json:"request"`
}

type EvaluationResponse struct {
	Version  string                       `json:"version" jsonschema:"enum=provider/v1"`
	Answers  map[string]evaluation.Answer `json:"answers"`
	Metadata map[string]any               `json:"metadata,omitempty"`
}

func (i Info) Supports(action string) bool { _, ok := i.manifest.Actions[action]; return ok }

func (i Info) ResolveModel(model string) (string, error) { return i.manifest.ResolveModel(model) }

func (i Info) Evaluate(ctx context.Context, request EvaluationRequest) (EvaluationResponse, error) {
	var response EvaluationResponse
	if err := request.Questions.Validate(); err != nil {
		return response, err
	}
	model, err := i.ResolveModel(request.Model)
	if err != nil {
		return response, err
	}
	request.Model = model
	plan, err := i.manifest.Render(ActionEvaluate, request)
	if err != nil {
		return response, err
	}
	plan = resolvePlanCommand(plan, i.path)
	if plan.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, plan.Timeout)
		defer cancel()
	}
	err = runJSON(ctx, plan, EvaluationEnvelope{Version: Protocol, Action: ActionEvaluate, Request: request}, &response, request.Dir)
	if ctx.Err() != nil {
		return response, ctx.Err()
	}
	if err != nil {
		return response, err
	}
	if response.Version != Protocol {
		return response, fmt.Errorf("invalid evaluation protocol %q", response.Version)
	}
	return response, request.Questions.Check(response.Answers, true)
}
