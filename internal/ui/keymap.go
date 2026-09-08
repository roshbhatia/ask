package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	shared "github.com/roshbhatia/go-utils/keymap"
)

var keys = shared.Must(
	shared.Binding{
		ID: "run-stop", Keys: []string{"ctrl+c", "esc"}, Display: "ctrl+c", Short: "stop",
		Description: "stop the active run", Contexts: []string{"run"},
	},
	shared.Binding{
		ID: "pick-up", Keys: []string{"up", "k"}, Display: "↑/k", Short: "up",
		Description: "select the previous provider", Contexts: []string{"pick"},
	},
	shared.Binding{
		ID: "pick-down", Keys: []string{"down", "j"}, Display: "↓/j", Short: "down",
		Description: "select the next provider", Contexts: []string{"pick"},
	},
	shared.Binding{
		ID: "pick-take", Keys: []string{"enter", "space"}, Display: "enter", Short: "run it",
		Description: "run the selected provider", Contexts: []string{"pick"},
	},
	shared.Binding{
		ID: "pick-number", Keys: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}, Display: "1-9", Short: "jump",
		Description: "run a provider by number", Contexts: []string{"pick"},
	},
	shared.Binding{
		ID: "pick-quit", Keys: []string{"ctrl+c", "esc", "q"}, Display: "q", Short: "quit",
		Description: "close the provider picker", Contexts: []string{"pick"},
	},
	shared.Binding{
		ID: "answer-take", Keys: []string{"enter"}, Display: "enter", Short: "answer",
		Description: "submit the answer", Contexts: []string{"answer"},
	},
	shared.Binding{
		ID: "answer-quit", Keys: []string{"ctrl+c", "esc"}, Display: "esc", Short: "give up",
		Description: "cancel the question", Contexts: []string{"answer"},
	},
)

func bubbleBinding(id string) key.Binding {
	binding, ok := keys.Binding(id)
	if !ok {
		panic(fmt.Sprintf("unknown UI binding %q", id))
	}
	hint, err := keys.Hint(id)
	if err != nil {
		panic(err)
	}
	bubbleKeys := append([]string(nil), binding.Keys...)
	for index, value := range bubbleKeys {
		if value == "space" {
			bubbleKeys[index] = " "
		}
	}
	return key.NewBinding(key.WithKeys(bubbleKeys...), key.WithHelp(hint.Keys, hint.Short))
}
