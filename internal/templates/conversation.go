package templates

import (
	"strings"
	"text/template"
)

var (
	clarificationPrompt = template.Must(template.New("clarification").Parse(`{{.Prompt}}

{{.Rule}}`))
	answeredQuestionPrompt = template.Must(template.New("answered-question").Parse(`{{.Prompt}}

You asked: {{.Question}}
The answer: {{.Answer}}`))
	rejectedAnswerPrompt = template.Must(template.New("rejected-answer").Parse(`{{.Prompt}}

Your last answer was rejected. {{.Reason}}
Answer again, in the shape asked for.`))
)

type conversationPrompt struct {
	Prompt   string
	Rule     string
	Question string
	Answer   string
	Reason   string
}

func WithClarificationRule(prompt, rule string) (string, error) {
	return renderConversation(clarificationPrompt, conversationPrompt{Prompt: prompt, Rule: rule})
}

func WithAnsweredQuestion(prompt, question, answer string) (string, error) {
	return renderConversation(answeredQuestionPrompt, conversationPrompt{
		Prompt:   prompt,
		Question: question,
		Answer:   answer,
	})
}

func WithRejectedAnswer(prompt, reason string) (string, error) {
	return renderConversation(rejectedAnswerPrompt, conversationPrompt{Prompt: prompt, Reason: reason})
}

func renderConversation(prompt *template.Template, data conversationPrompt) (string, error) {
	var rendered strings.Builder
	if err := prompt.Execute(&rendered, data); err != nil {
		return "", err
	}
	return rendered.String(), nil
}
