package console

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mroy31/gonetem/internal/options"
	"github.com/mroy31/gonetem/internal/proto"
)

type promptErrorMsg string

// promptProjectPathMsg carries new project path after a saveAs command
type promptProjectPathMsg string

// promptOutputMsg carries command output produced after the command line
// was submitted (like the save completion) to display in the transcript.
type promptOutputMsg string

// promptProgressMsg carries a transient progress line displayed by the
// prompt while a long running command is in progress. An empty message
// clears the progress display.
type promptProgressMsg string

// promptClearMsg asks the prompt to empty the transcript, as requested by
// the clear command. It must be delivered through a command (tea.Cmd)
// returned by Update: commands run inside the event loop, where a direct
// Program.Send would block forever on the unbuffered message channel.
type promptClearMsg struct{}

// promptBusyMsg freezes the prompt input while an asynchronous command
// (run, reload) is modifying the topology: busy on start, false on
// completion. message is displayed above the progress bars while the
// command runs. Both transitions are sent from the background goroutine
// consuming the command stream, which keeps their order consistent with
// the progress messages it also sends.
type promptBusyMsg struct {
	busy    bool
	message string
}

// editorFinishedMsg is sent to the prompt when the external editor spawned
// by a command (like edit) exits. finish uploads or discards the edited
// data and may return an error to display below the prompt.
type editorFinishedMsg struct {
	err    error
	finish func(error) error
}

// promptReserveLines is the number of lines the main prompt keeps free
// below it, above the bottom of the terminal: room for the command errors
// and completion suggestions without ever moving the prompt, and the
// project path on the last line.
const promptReserveLines = 6

type teaPromptModel struct {
	input       textinput.Model
	prefix      string
	completions func(string) []Completion
	onSubmit    func(string) (bool, error)
	// subPrompt is set when a command requests an additional input (like the
	// config file selection of viewConfig) and takes over the prompt line
	// until an answer is submitted or the user cancels it
	subPrompt        *inputPromptRequest
	savedCompletions func(string) []Completion
	// takeOutput drains the command outputs queued while the sub prompt
	// callback was running
	takeOutput func() []string
	// pendingCmd is set by a submitted command to be run by the program
	// right after: the external topology editor, with the terminal taken
	// back from the TUI until it exits, or a function sending a message
	// back to the model like the transcript clear
	pendingCmd   tea.Cmd
	history      []string
	historyIndex int
	historyDraft string
	// promptError holds the last command error, rendered in red just below
	// the prompt line until the next submission
	promptError string
	transcript  []string
	// scrollOffset is the number of transcript lines hidden below the
	// bottom of the screen, used to scroll back through a long history
	scrollOffset int
	progress     string
	// busy is set while an asynchronous command (run, reload) runs: the
	// input is frozen and only the transcript keeps scrolling
	busy bool
	// busyMessage is the text shown above the progress bars while busy,
	// naming the running command and asking the user to wait
	busyMessage string
	showMatches bool
	projectName string
	projectPath string
	height      int
	fullscreen  bool
	showHeader  bool
	done        bool
	result      string
}

// newStyledTextInput creates the focused text input shared by all prompt
// models: colored prompt sign and dimmed completion suggestions.
func newStyledTextInput(prefix string) textinput.Model {
	input := textinput.New()
	input.Prompt = prefix
	input.ShowSuggestions = true

	styles := input.Styles()
	styles.Focused.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	styles.Focused.Suggestion = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	input.SetStyles(styles)
	return input
}

func newTeaPromptModel(prefix string, completions func(string) []Completion, onSubmit func(string) (bool, error)) *teaPromptModel {
	return &teaPromptModel{
		input:        newStyledTextInput(prefix),
		prefix:       prefix,
		completions:  completions,
		onSubmit:     onSubmit,
		historyIndex: -1,
		showMatches:  true,
	}
}

func (m *teaPromptModel) Init() tea.Cmd {
	return m.input.Focus()
}

func (m *teaPromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case promptErrorMsg:
		m.promptError = string(msg)
		return m, nil
	case promptOutputMsg:
		m.appendOutput(string(msg))
		return m, nil
	case promptProgressMsg:
		m.progress = string(msg)
		return m, nil
	case promptClearMsg:
		m.clearTranscript()
		return m, nil
	case promptProjectPathMsg:
		m.projectPath = string(msg)
		return m, nil
	case promptBusyMsg:
		m.busy = msg.busy
		m.busyMessage = msg.message
		if m.busy {
			// render the input blurred, without cursor, while frozen
			m.input.Blur()
			return m, nil
		}
		return m, m.input.Focus()
	case editorFinishedMsg:
		if msg.finish != nil {
			err := msg.finish(msg.err)
			if err != nil {
				m.promptError = err.Error()
			}
		}
		m.drainOutput()
		return m, nil
	case tea.WindowSizeMsg:
		m.height = msg.Height
		return m, nil
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			m.scrollBy(3)
			return m, nil
		case tea.MouseWheelDown:
			m.scrollBy(-3)
			return m, nil
		}
	case tea.KeyPressMsg:
		if m.busy {
			// while an asynchronous command runs, the input is frozen:
			// only scrolling the transcript keeps working
			switch msg.String() {
			case "pgup":
				m.scrollBy(m.transcriptPageHeight())
			case "pgdown":
				m.scrollBy(-m.transcriptPageHeight())
			}
			return m, nil
		}

		switch msg.String() {
		case "esc":
			if m.subPrompt != nil {
				m.endSubPrompt()
				return m, nil
			}

			// clear error
			m.promptError = ""
			m.resetInput()
			return m, nil
		case "enter":
			value := m.input.Value()
			if m.subPrompt != nil {
				request := m.subPrompt
				echo := m.input.Prompt + value
				m.endSubPrompt()
				m.promptError = ""
				m.scrollOffset = 0
				if value != "" {
					m.transcript = append(m.transcript, echo)
					request.submit(value)
				}
				m.drainOutput()
				m.updateCompletions()
				return m, nil
			}
			m.result = value
			if m.onSubmit != nil {
				m.promptError = ""
				m.scrollOffset = 0
				if value != "" {
					m.transcript = append(m.transcript, m.input.Prompt+value)
				}
				done, err := m.onSubmit(value)
				if err != nil {
					m.promptError = err.Error()
				}
				if done {
					m.done = true
					return m, tea.Quit
				}
				if value != "" {
					m.history = append(m.history, value)
				}
				m.resetInput()
				// a command may hand over a program command to run right
				// after the submission: an external editor taking the
				// terminal back, or a message back to the model
				nextCmd := m.pendingCmd
				m.pendingCmd = nil
				return m, nextCmd
			}
			if value != "" {
				m.transcript = append(m.transcript, m.input.Prompt+value)
			}
			m.done = true
			return m, tea.Quit
		case "pgup":
			m.scrollBy(m.transcriptPageHeight())
			return m, nil
		case "pgdown":
			m.scrollBy(-m.transcriptPageHeight())
			return m, nil
		case "tab":
			if strings.TrimSpace(m.input.Value()) != "" && m.completions != nil {
				matches := m.completions(m.input.Value())
				if len(matches) > 0 {
					m.input.SetValue(matches[0].Text)
					m.input.CursorEnd()
					m.updateCompletions()
					return m, nil
				}
			}
		case "up":
			if m.subPrompt == nil && m.moveHistory(-1) {
				m.updateCompletions()
				return m, nil
			}
		case "down":
			if m.subPrompt == nil && m.moveHistory(1) {
				m.updateCompletions()
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.updateCompletions()
	return m, cmd
}

// startSubPrompt displays the additional input request of a command in
// place of the main prompt, with its own prefix and completions, until an
// answer is submitted or the user cancels it.
func (m *teaPromptModel) startSubPrompt(request *inputPromptRequest) {
	m.subPrompt = request
	m.savedCompletions = m.completions
	m.completions = prefixMatcher(request.completions)
	m.input.Prompt = request.prefix
	m.input.Reset()
	m.historyIndex = -1
	m.historyDraft = ""
	m.updateCompletions()
}

func (m *teaPromptModel) endSubPrompt() {
	m.subPrompt = nil
	m.completions = m.savedCompletions
	m.savedCompletions = nil
	m.input.Prompt = m.prefix
	m.resetInput()
}

// scrollBy scrolls the transcript back (positive delta shows older lines) or
// forward. The offset is kept in [0, len(transcript)] here, and clamped
// again in View to the number of lines that really overflow the screen.
func (m *teaPromptModel) scrollBy(delta int) {
	m.scrollOffset += delta
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

// transcriptPageHeight approximates the number of transcript lines visible
// without scrolling, to scroll by whole pages with pgup/pgdown.
func (m *teaPromptModel) transcriptPageHeight() int {
	headerLines := 0
	if m.showHeader {
		headerLines = 4
	}
	// one page of the transcript is what View makes visible: the terminal
	// minus the header, the prompt line and the block reserved below it
	return max(1, m.height-headerLines-1-promptReserveLines)
}

func (m *teaPromptModel) moveHistory(direction int) bool {
	if len(m.history) == 0 {
		return false
	}

	if m.historyIndex == -1 {
		if direction > 0 {
			return false
		}
		m.historyDraft = m.input.Value()
		m.historyIndex = len(m.history)
	}

	m.historyIndex += direction
	if m.historyIndex < 0 {
		m.historyIndex = 0
	}
	if m.historyIndex >= len(m.history) {
		m.historyIndex = -1
		m.input.SetValue(m.historyDraft)
		return true
	}
	m.input.SetValue(m.history[m.historyIndex])
	return true
}

// resetInput clears the prompt line, its suggestions and the history
// navigation, ready for the next command or sub prompt.
func (m *teaPromptModel) resetInput() {
	m.input.Reset()
	m.input.SetSuggestions(nil)
	m.historyIndex = -1
	m.historyDraft = ""
	m.updateCompletions()
}

// drainOutput moves the outputs queued by a command, or by its sub prompt
// callback, into the transcript.
func (m *teaPromptModel) drainOutput() {
	if m.takeOutput == nil {
		return
	}
	for _, output := range m.takeOutput() {
		m.appendOutput(output)
	}
}

func (m *teaPromptModel) updateCompletions() {
	if m.completions == nil || strings.TrimSpace(m.input.Value()) == "" {
		m.input.SetSuggestions(nil)
		return
	}
	m.input.SetSuggestions(completionTexts(m.completions(m.input.Value())))
}

func (m *teaPromptModel) appendOutput(output string) {
	output = strings.TrimSuffix(output, "\n")
	if output == "" {
		return
	}
	m.transcript = append(m.transcript, strings.Split(output, "\n")...)
	// new output brings the user back to the bottom of the transcript
	m.scrollOffset = 0
}

// clearTranscript empties the transcript and resets scrolling, as requested
// by the clear command.
func (m *teaPromptModel) clearTranscript() {
	m.transcript = nil
	m.scrollOffset = 0
}

func (m *teaPromptModel) View() tea.View {
	header := make([]string, 0, 4)
	if m.showHeader {
		bannerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
		titleStyle := lipgloss.NewStyle().Bold(true)
		header = append(header,
			bannerStyle.Render("Welcome to gonetem console "+options.VERSION),
			titleStyle.Render("Project: "+m.projectName),
			"Please use `exit` to close the project",
			"",
		)
	}

	var suggestions []string
	if !m.done && m.showMatches && m.completions != nil && strings.TrimSpace(m.input.Value()) != "" {
		suggestions = renderSuggestions(m.completions(m.input.Value()))
	}

	var inputLines []string
	if !m.done {
		inputLines = append(inputLines, m.input.View())
	}

	var progressLines []string
	if !m.done && m.busy && m.busyMessage != "" {
		style := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11"))
		progressLines = append(progressLines, style.Render(m.busyMessage))
	}
	if !m.done && m.progress != "" {
		style := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
		for _, line := range strings.Split(m.progress, "\n") {
			progressLines = append(progressLines, style.Render(line))
		}
	}

	var errorLines []string
	if !m.done && m.promptError != "" {
		for _, line := range strings.Split(m.promptError, "\n") {
			errorLines = append(errorLines, lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(line))
		}
	}

	bodySpace := m.height - len(header)
	if m.height > 0 {
		bodySpace = max(0, bodySpace)
	}

	// the prompt is pinned promptReserveLines lines above the bottom of the
	// terminal: a fixed block below it holds the command errors, the
	// completion suggestions and the project path on the last line, so the
	// prompt never moves when suggestions appear or disappear
	footer := ""
	if m.showHeader && !m.done {
		path := m.projectPath
		if path == "" {
			path = "Unavailable (attached to a running project)"
		}
		footer = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Path: " + path)
	}
	footerLines := 0
	if footer != "" {
		footerLines = 1
	}
	underPrompt := max(promptReserveLines, len(errorLines)+footerLines)
	if m.height > 0 {
		suggestionSpace := max(0, underPrompt-footerLines-len(errorLines))
		suggestions = suggestions[:min(len(suggestions), suggestionSpace)]
	}

	transcriptSpace := len(m.transcript)
	if m.height > 0 {
		transcriptSpace = max(0, bodySpace-len(inputLines)-len(progressLines)-underPrompt)
	}

	// window the transcript into the visible space, scrolled by scrollOffset
	// lines from its end; a scrolled view spends one line for the hint about
	// the hidden lines
	scrollOffset := m.scrollOffset
	var indicatorLines []string
	if m.height > 0 {
		if scrollOffset > 0 {
			transcriptSpace = max(0, transcriptSpace-1)
			scrollOffset = min(scrollOffset, max(0, len(m.transcript)-transcriptSpace))
		}
		if scrollOffset > 0 {
			indicatorLines = append(indicatorLines, lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(
				fmt.Sprintf("  ↑ %d/%d lines above (pgup/pgdown, wheel to scroll)",
					scrollOffset, len(m.transcript)-transcriptSpace)))
		} else {
			scrollOffset = 0
		}
	}
	transcriptEnd := max(0, len(m.transcript)-scrollOffset)
	transcript := m.transcript[max(0, transcriptEnd-transcriptSpace):transcriptEnd]

	lines := make([]string, 0, len(header)+bodySpace)
	lines = append(lines, header...)
	lines = append(lines, indicatorLines...)
	lines = append(lines, transcript...)
	// keep the prompt anchored above its reserved block: pad the gap
	// between the transcript and the prompt when the transcript does not
	// fill the terminal
	gap := 0
	if m.height > 0 {
		gap = max(0, m.height-underPrompt-len(header)-len(indicatorLines)-len(transcript)-
			len(progressLines)-len(inputLines))
	}
	lines = append(lines, make([]string, gap)...)
	lines = append(lines, progressLines...)
	lines = append(lines, inputLines...)
	lines = append(lines, errorLines...)
	lines = append(lines, suggestions...)
	if m.height > 0 {
		// pad the block reserved below the prompt, leaving the last line
		// for the project path when it is displayed
		for len(lines) < m.height-footerLines {
			lines = append(lines, "")
		}
	}
	if footer != "" {
		lines = append(lines, footer)
	}

	view := tea.NewView(strings.Join(lines, "\n"))
	view.AltScreen = m.fullscreen
	if m.fullscreen {
		// capture the mouse wheel to scroll the transcript, instead of the
		// terminal native scrollback which is empty in the alt screen
		view.MouseMode = tea.MouseModeCellMotion
	}
	return view
}

// prefixMatcher returns a completion function filtering the given
// completions with a case insensitive prefix match on the input.
func prefixMatcher(completions []Completion) func(string) []Completion {
	return func(input string) []Completion {
		matches := make([]Completion, 0, len(completions))
		for _, completion := range completions {
			if strings.HasPrefix(strings.ToLower(completion.Text), strings.ToLower(input)) {
				matches = append(matches, completion)
			}
		}
		return matches
	}
}

// completionTexts extracts the texts of completions, to display as
// suggestions under the text input.
func completionTexts(completions []Completion) []string {
	texts := make([]string, 0, len(completions))
	for _, completion := range completions {
		texts = append(texts, completion.Text)
	}
	return texts
}

// renderSuggestions renders up to 5 completions, the first one highlighted
// as the value tab will complete to.
func renderSuggestions(matches []Completion) []string {
	limit := min(len(matches), 5)
	lines := make([]string, 0, limit)
	for i, match := range matches[:limit] {
		line := "  " + match.Text
		if match.Description != "" {
			line += "  " + match.Description
		}
		fg := "8"
		if i == 0 {
			fg = "10"
		}
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color(fg)).Render(line))
	}
	return lines
}

// teaSimplePromptModel is a simplified one-shot question model shared by the
// clean and connect command prompts. Unlike teaPromptModel, it has no
// history, no transcript and no command execution: it only reads a single
// answer with completions, then quits.
type teaSimplePromptModel struct {
	input       textinput.Model
	completions func(string) []Completion
	done        bool
	result      string
}

func newTeaSimplePromptModel(prefix string, completions []Completion) *teaSimplePromptModel {
	model := &teaSimplePromptModel{
		input:       newStyledTextInput(prefix),
		completions: prefixMatcher(completions),
	}
	model.updateCompletions()
	return model
}

func (m *teaSimplePromptModel) Init() tea.Cmd {
	return m.input.Focus()
}

func (m *teaSimplePromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch msg.String() {
		case "ctrl+c", "esc":
			m.done = true
			return m, tea.Quit
		case "enter":
			m.result = m.input.Value()
			m.done = true
			return m, tea.Quit
		case "tab":
			if strings.TrimSpace(m.input.Value()) != "" {
				if matches := m.completions(m.input.Value()); len(matches) > 0 {
					m.input.SetValue(matches[0].Text)
					m.input.CursorEnd()
					m.updateCompletions()
					return m, nil
				}
			}
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.updateCompletions()
	return m, cmd
}

func (m *teaSimplePromptModel) updateCompletions() {
	if m.completions == nil {
		m.input.SetSuggestions(nil)
		return
	}
	m.input.SetSuggestions(completionTexts(m.completions(m.input.Value())))
}

func (m *teaSimplePromptModel) View() tea.View {
	if m.done {
		return tea.NewView("")
	}

	// always show completion suggestions
	lines := []string{m.input.View()}
	lines = append(lines, renderSuggestions(m.completions(m.input.Value()))...)

	return tea.NewView(strings.Join(lines, "\n"))
}

// teaCleanPromptModel is the prompt model used by the `clean` command.
// It asks the user to confirm the prune of unused containers with yes/no.
type teaCleanPromptModel struct {
	*teaSimplePromptModel
}

func newTeaCleanPromptModel() *teaCleanPromptModel {
	return &teaCleanPromptModel{
		teaSimplePromptModel: newTeaSimplePromptModel(
			"Are you sure you want prune unused containers ? ",
			ConfirmationCompletions(),
		),
	}
}

// the Update override keeps the outer model visible to RunCleanPrompt when
// the program replaces its model on the first update.
func (m *teaCleanPromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := m.teaSimplePromptModel.Update(msg)
	return m, cmd
}

// confirmed returns true when the submitted answer is yes.
func (m *teaCleanPromptModel) confirmed() bool {
	answer := strings.ToLower(strings.TrimSpace(m.result))
	return answer == "yes" || answer == "y"
}

// RunCleanPrompt asks the user to confirm the clean command.
// It returns true only when the user answered yes.
func RunCleanPrompt() (bool, error) {
	model := newTeaCleanPromptModel()

	finalModel, err := tea.NewProgram(model).Run()
	if err != nil {
		return false, fmt.Errorf("run prompt: %w", err)
	}
	return finalModel.(*teaCleanPromptModel).confirmed(), nil
}

// teaConnectPromptModel is the prompt model used by the `connect` command.
// It lets the user select a project running on the server.
type teaConnectPromptModel struct {
	*teaSimplePromptModel
}

func newTeaConnectPromptModel(projects *proto.PrjListResponse) *teaConnectPromptModel {
	return &teaConnectPromptModel{
		teaSimplePromptModel: newTeaSimplePromptModel(
			"Select project: ",
			ProjectCompletions(projects),
		),
	}
}

// the Update override keeps the outer model visible to RunConnectPrompt when
// the program replaces its model on the first update.
func (m *teaConnectPromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := m.teaSimplePromptModel.Update(msg)
	return m, cmd
}

// RunConnectPrompt lets the user select one of the projects running
// on the server and returns the selected project name.
func RunConnectPrompt(projects *proto.PrjListResponse) (string, error) {
	model := newTeaConnectPromptModel(projects)

	finalModel, err := tea.NewProgram(model).Run()
	if err != nil {
		return "", fmt.Errorf("run prompt: %w", err)
	}
	return finalModel.(*teaConnectPromptModel).result, nil
}
