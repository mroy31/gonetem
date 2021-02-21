package console

import (
	"fmt"

	"github.com/c-bata/go-prompt"
	"github.com/mroy31/gonetem/internal/proto"
)

type PromptCompleter struct {
}

func (c *PromptCompleter) Complete(d prompt.Document) []prompt.Suggest {
	return []prompt.Suggest{}
}

func NewPromptCompleter() *PromptCompleter {
	return &PromptCompleter{}
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
