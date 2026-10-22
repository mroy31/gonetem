package console

import (
	"reflect"
	"testing"
)

func TestPromptCompleterComplete(t *testing.T) {
	completer := NewPromptCompleter(&NetemPrompt{
		commands: map[string]*NetemCommand{
			"start":  {},
			"status": {},
			"stop":   {},
		},
		nodes: []NetemNode{{Name: "router1"}, {Name: "router2"}},
	})

	tests := []struct {
		input string
		want  []string
	}{
		{input: "st", want: []string{"start", "status", "stop"}},
		{input: "start rou", want: []string{"start router1", "start router2"}},
		{input: "restart ", want: []string{"restart router1", "restart router2"}},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got := completer.Complete(test.input)
			texts := make([]string, len(got))
			for i, completion := range got {
				texts[i] = completion.Text
			}
			if !reflect.DeepEqual(texts, test.want) {
				t.Fatalf("Complete(%q) = %v, want %v", test.input, texts, test.want)
			}
		})
	}
}
