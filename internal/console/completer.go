package console

import (
	"regexp"
	"sort"
	"strings"

	"github.com/mroy31/gonetem/internal/proto"
)

// ifIndexRe extracts the numeric index ending an interface name returned by
// the project status: ethN on docker nodes, <bridge>.N on ovs nodes. Names
// without index, like the mgnt interface, do not match: they are not
// addressable by the capture and ifState commands.
var ifIndexRe = regexp.MustCompile(`(\d+)$`)

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

	if len(previousArgs) != 1 &&
		!(len(previousArgs) == 2 && previousArgs[0] == "ifState") {
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
	case "capture":
		// capture <node_name>.<if_number>
		matches = c.ifCompletions(prefix, word)
	case "ifState":
		if len(previousArgs) == 1 {
			// ifState <node_name>.<if_number>
			matches = c.ifCompletions(prefix, word)
		} else {
			// ifState <node_name>.<if_number> up|down
			for _, state := range []string{"up", "down"} {
				if strings.HasPrefix(state, word) {
					matches = append(matches,
						Completion{Text: prefix + state})
				}
			}
			sort.Slice(matches, func(i, j int) bool {
				return matches[i].Text < matches[j].Text
			})
		}
	}
	return matches
}

// ifCompletions completes the <node_name>.<if_number> argument expected by
// the capture and ifState commands. Each running node interface provides one
// candidate, using the numeric index extracted from its name.
func (c *PromptCompleter) ifCompletions(prefix, word string) []Completion {
	var matches []Completion
	for _, node := range c.prt.nodeList() {
		for _, ifName := range node.Interfaces {
			ifIndex := ifIndexRe.FindString(ifName)
			if ifIndex == "" {
				continue
			}
			candidate := node.Name + "." + ifIndex
			if strings.HasPrefix(candidate, word) {
				matches = append(matches, Completion{Text: prefix + candidate})
			}
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Text < matches[j].Text
	})
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
