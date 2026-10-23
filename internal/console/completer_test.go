package console

import (
	"reflect"
	"testing"
)

func TestPromptCompleterComplete(t *testing.T) {
	completer := NewPromptCompleter(&NetemPrompt{
		commands: map[string]*NetemCommand{
			"capture": {},
			"ifState": {},
			"start":   {},
			"status":  {},
			"stop":    {},
		},
		nodes: []NetemNode{
			{Name: "router1", Interfaces: []string{"eth0", "eth1", "mgnt"}},
			{Name: "router2", Interfaces: []string{"eth0", "mgnt"}},
		},
	})

	tests := []struct {
		input string
		want  []string
	}{
		{input: "st", want: []string{"start", "status", "stop"}},
		{input: "start rou", want: []string{"start router1", "start router2"}},
		{input: "restart ", want: []string{"restart router1", "restart router2"}},
		{input: "cap", want: []string{"capture"}},
		{input: "capture ", want: []string{
			"capture router1.0", "capture router1.1", "capture router2.0"}},
		{input: "capture router1.", want: []string{
			"capture router1.0", "capture router1.1"}},
		{input: "capture router2.1", want: []string{}},
		{input: "ifState ", want: []string{
			"ifState router1.0", "ifState router1.1", "ifState router2.0"}},
		{input: "ifState router2.0 ", want: []string{
			"ifState router2.0 down", "ifState router2.0 up"}},
		{input: "ifState router2.0 u", want: []string{"ifState router2.0 up"}},
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
