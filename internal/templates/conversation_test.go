package templates

import "testing"

func TestConversationPromptTemplates(t *testing.T) {
	tests := []struct {
		name string
		run  func() (string, error)
		want string
	}{
		{
			name: "clarification rule",
			run: func() (string, error) {
				return WithClarificationRule("Review this.", "Ask one question.")
			},
			want: `Review this.

Ask one question.`,
		},
		{
			name: "answered question",
			run: func() (string, error) {
				return WithAnsweredQuestion("Review this.", "Which branch?", "main")
			},
			want: `Review this.

You asked: Which branch?
The answer: main`,
		},
		{
			name: "rejected answer",
			run: func() (string, error) {
				return WithRejectedAnswer("Review this.", "name: required")
			},
			want: `Review this.

Your last answer was rejected. name: required
Answer again, in the shape asked for.`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.run()
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}
