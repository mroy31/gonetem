package console

import (
	"fmt"
	"strings"

	"github.com/c-bata/go-prompt"
	"github.com/mroy31/gonetem/internal/proto"
)

type PromptCompleter struct {
	prt *NetemPrompt
}

func (c *PromptCompleter) Complete(d prompt.Document) []prompt.Suggest {
	if d.TextBeforeCursor() == "" {
		return []prompt.Suggest{}
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

		return suggestions
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

			return suggestions
		}
	}

	return []prompt.Suggest{}
}

func NewPromptCompleter(prompt *NetemPrompt) *PromptCompleter {
	return &PromptCompleter{prt: prompt}
}

type ConnectCompleter struct {
	projects *proto.PrjListResponse
}

func (c *ConnectCompleter) Complete(d prompt.Document) []prompt.Suggest {
	suggestions := []prompt.Suggest{}
	for _, prj := range c.projects.GetProjects() {
		suggestions = append(suggestions, prompt.Suggest{
			Text:        prj.GetName(),
			Description: fmt.Sprintf("Open at %s", prj.GetOpenAt()),
		})
	}
	return suggestions
}

func NewConnectCompleter(projects *proto.PrjListResponse) *ConnectCompleter {
	return &ConnectCompleter{projects}
}

func ConfirmComplete(d prompt.Document) []prompt.Suggest {
	return []prompt.Suggest{
		{
			Text: "yes",
		},
		{
			Text: "no",
		},
	}
}
