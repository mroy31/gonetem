package console

import (
	"sort"
	"strings"

	"github.com/mroy31/gonetem/internal/proto"
)

type Completion struct {
	Text        string
	Description string
}

type PromptCompleter struct {
	prt *NetemPrompt
}

func (c *PromptCompleter) Complete(input string) []Completion {
	wordStart := strings.LastIndex(input, " ") + 1
	prefix := input[:wordStart]
	word := input[wordStart:]
	previousArgs := strings.Fields(prefix)

	var matches []Completion
	if len(previousArgs) == 0 {
		for name, command := range c.prt.commands {
			if strings.HasPrefix(name, word) {
				matches = append(matches, Completion{
					Text:        prefix + name,
					Description: command.Desc,
				})
			}
		}
		sort.Slice(matches, func(i, j int) bool {
			return matches[i].Text < matches[j].Text
		})
		return matches
	}

	if len(previousArgs) != 1 {
		return nil
	}

	switch previousArgs[0] {
	case "console", "start", "stop", "restart", "shell", "viewConfig":
		for _, node := range c.prt.nodeList() {
			if strings.HasPrefix(node.Name, word) {
				matches = append(matches, Completion{Text: prefix + node.Name})
			}
		}
		sort.Slice(matches, func(i, j int) bool {
			return matches[i].Text < matches[j].Text
		})
		if previousArgs[0] != "restart" && strings.HasPrefix("all", word) {
			matches = append(matches, Completion{Text: prefix + "all"})
		}
	}
	return matches
}

func NewPromptCompleter(prompt *NetemPrompt) *PromptCompleter {
	return &PromptCompleter{prt: prompt}
}

func ProjectCompletions(projects *proto.PrjListResponse) []Completion {
	var completions []Completion
	for _, project := range projects.GetProjects() {
		completions = append(completions, Completion{
			Text:        project.GetName(),
			Description: "Open at " + project.GetOpenAt(),
		})
	}
	return completions
}

func ConfigurationCompletions(files *proto.ConfigFilesResponse) []Completion {
	var completions []Completion
	for _, file := range files.GetFiles() {
		completions = append(completions, Completion{Text: file.GetName()})
	}
	return completions
}

func ConfirmationCompletions() []Completion {
	return []Completion{{Text: "yes"}, {Text: "no"}}
}
