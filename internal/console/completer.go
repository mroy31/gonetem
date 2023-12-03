package console

import (
	"fmt"
	"strings"

	prompt "github.com/elk-language/go-prompt"
	istrings "github.com/elk-language/go-prompt/strings"
	"github.com/mroy31/gonetem/internal/proto"
)

type PromptCompleter struct {
	prt *NetemPrompt
}

func (c *PromptCompleter) Complete(d prompt.Document) ([]prompt.Suggest, istrings.RuneNumber, istrings.RuneNumber) {
	endIndex := d.CurrentRuneIndex()
	if d.TextBeforeCursor() == "" {
		return []prompt.Suggest{}, 0, 0
	}
	args := strings.Split(d.TextBeforeCursor(), " ")

	if len(args) == 1 {
		suggestions := make([]prompt.Suggest, 0)
		for n, cmd := range c.prt.commands {
			if !strings.HasPrefix(n, args[0]) {
				continue
			}
			suggestions = append(suggestions, prompt.Suggest{
				Text:        n,
				Description: cmd.Desc,
			})
		}

		startIndex := endIndex - istrings.RuneCountInString(args[0])
		return suggestions, startIndex, endIndex
	}

	if len(args) == 2 {
		switch args[0] {
		case "console", "start", "stop", "restart", "shell":
			suggestions := make([]prompt.Suggest, 0)
			for _, n := range c.prt.nodes {
				if !strings.HasPrefix(n.Name, args[1]) {
					continue
				}
				suggestions = append(suggestions, prompt.Suggest{Text: n.Name})
			}

			if args[0] == "console" || args[0] == "shell" {
				suggestions = append(suggestions, prompt.Suggest{Text: "all"})

			}

			startIndex := endIndex - istrings.RuneCountInString(args[1])
			return suggestions, startIndex, endIndex
		}
	}

	return []prompt.Suggest{}, 0, 0
}

func NewPromptCompleter(prompt *NetemPrompt) *PromptCompleter {
	return &PromptCompleter{prt: prompt}
}

type ConnectCompleter struct {
	projects *proto.PrjListResponse
}

func (c *ConnectCompleter) Complete(d prompt.Document) ([]prompt.Suggest, istrings.RuneNumber, istrings.RuneNumber) {
	endIndex := d.CurrentRuneIndex()
	w := d.GetWordBeforeCursor()

	suggestions := []prompt.Suggest{}
	for _, prj := range c.projects.GetProjects() {
		suggestions = append(suggestions, prompt.Suggest{
			Text:        prj.GetName(),
			Description: fmt.Sprintf("Open at %s", prj.GetOpenAt()),
		})
	}

	startIndex := endIndex - istrings.RuneCountInString(w)
	return prompt.FilterHasPrefix(suggestions, w, true), startIndex, endIndex
}

func NewConnectCompleter(projects *proto.PrjListResponse) *ConnectCompleter {
	return &ConnectCompleter{projects}
}

func ConfirmComplete(d prompt.Document) ([]prompt.Suggest, istrings.RuneNumber, istrings.RuneNumber) {
	endIndex := d.CurrentRuneIndex()
	w := d.GetWordBeforeCursor()
	startIndex := endIndex - istrings.RuneCountInString(w)

	return prompt.FilterHasPrefix([]prompt.Suggest{
		{
			Text: "yes",
		},
		{
			Text: "no",
		},
	}, w, true), startIndex, endIndex
}
