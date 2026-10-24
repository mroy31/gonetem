package console

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mroy31/gonetem/internal/options"
	"github.com/mroy31/gonetem/internal/proto"
	"go.dalton.dog/bubbleup/v2"
)

type promptErrorMsg string

// promptAlertMsg carries message to be displayed as an alert
// it can be async command output or errors
type promptAlertMsg struct {
	alertType string
	alertMsg  string
}

// promptProjectPathMsg carries new project path after a saveAs command
type promptProjectPathMsg string

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
// (run, reload, exit) is modifying the topology: busy on start, false on
// completion. message is displayed above the progress bars while the
// command runs. Both transitions are sent from the background goroutine
// consuming the command stream, which keeps their order consistent with
// the progress messages it also sends.
type promptBusyMsg struct {
	busy    bool
	message string
}

// promptQuitMsg asks the prompt to leave the TUI, sent by a final long
// running command once it completed, like the project close of the exit
// command. The model marks itself done, so the final frame drops the input,
// and quits the program.
type promptQuitMsg struct{}

// editorFinishedMsg is sent to the prompt when the external editor spawned
// by a command (like edit) exits. finish uploads or discards the edited
// data and may return an error to display below the prompt.
type editorFinishedMsg struct {
	err    error
	finish func(error) error
}

// promptSuggestionLines is the number of lines reserved above the prompt
// line, for the completion suggestions or the progress bars of a running
// command, so the prompt keeps a fixed position.
const promptSuggestionLines = 5

type promptModel struct {
	input       textinput.Model
	alert       bubbleup.AlertModel
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
	// startCmd is run once when the program starts, to launch a long
	// running command that reports its progress to the model, like the
	// topology start of the open command
	startCmd tea.Cmd
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
	// transcriptVP scrolls the command history in the space left between
	// the history banner and the footer: it windows the transcript lines,
	// soft wraps the long ones and keeps the scroll position
	transcriptVP viewport.Model
	// transcriptToBottom is set when new output, a submission or a clear
	// asks the transcript view to snap back to its last line
	transcriptToBottom bool
	progress           string
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
	width       int
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

func newStyledAlert() bubbleup.AlertModel {
	return bubbleup.NewAlertModel(70, true, 6*time.Second).
		WithMinWidth(20). // Dynamic width: alerts will size 20-70 chars
		WithPosition(bubbleup.BottomCenterPosition).
		WithAllowEscToClose()
}

func newPromptModel(prefix string, completions func(string) []Completion, onSubmit func(string) (bool, error)) *promptModel {
	model := &promptModel{
		input:        newStyledTextInput(prefix),
		alert:        newStyledAlert(),
		prefix:       prefix,
		completions:  completions,
		onSubmit:     onSubmit,
		historyIndex: -1,
		showMatches:  true,
	}
	// long transcript lines soft wrap in the viewport instead of overflowing
	// and corrupting the fixed-height layout; New also sets the scroll step
	// defaults, as the wheel delta
	model.transcriptVP = viewport.New()
	model.transcriptVP.SoftWrap = true
	// the first view shows the bottom of the history, where the prompt is
	model.transcriptToBottom = true
	return model
}

func (m *promptModel) Init() tea.Cmd {
	return tea.Batch(m.input.Focus(), m.alert.Init(), m.startCmd)
}

func (m *promptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case promptErrorMsg:
		m.promptError = string(msg)
		return m, nil
	case promptAlertMsg:
		alertCmd := m.alert.NewAlertCmd(msg.alertType, string(msg.alertMsg))
		return m, alertCmd
	case promptProgressMsg:
		m.progress = string(msg)
		return m, nil
	case promptClearMsg:
		m.clearTranscript()
		return m, nil
	case promptQuitMsg:
		m.done = true
		return m, tea.Quit
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
		m.width = msg.Width
		return m, nil
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			m.transcriptVP.ScrollUp(m.transcriptVP.MouseWheelDelta)
			return m, nil
		case tea.MouseWheelDown:
			m.transcriptVP.ScrollDown(m.transcriptVP.MouseWheelDelta)
			return m, nil
		}
	case tea.KeyPressMsg:
		if m.busy {
			// while an asynchronous command runs, the input is frozen:
			// only scrolling the transcript keeps working
			switch msg.String() {
			case "pgup":
				m.transcriptVP.PageUp()
			case "pgdown":
				m.transcriptVP.PageDown()
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
				m.transcriptToBottom = true
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
				m.transcriptToBottom = true
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
			m.transcriptVP.PageUp()
			return m, nil
		case "pgdown":
			m.transcriptVP.PageDown()
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

	var outInputCmd tea.Cmd
	m.input, outInputCmd = m.input.Update(msg)

	outAlert, outAlertCmd := m.alert.Update(msg)
	m.alert = outAlert.(bubbleup.AlertModel)

	m.updateCompletions()
	return m, tea.Batch(outInputCmd, outAlertCmd)
}

// startSubPrompt displays the additional input request of a command in
// place of the main prompt, with its own prefix and completions, until an
// answer is submitted or the user cancels it.
func (m *promptModel) startSubPrompt(request *inputPromptRequest) {
	m.subPrompt = request
	m.savedCompletions = m.completions
	m.completions = prefixMatcher(request.completions)
	m.input.Prompt = request.prefix
	m.input.Reset()
	m.historyIndex = -1
	m.historyDraft = ""
	m.updateCompletions()
}

func (m *promptModel) endSubPrompt() {
	m.subPrompt = nil
	m.completions = m.savedCompletions
	m.savedCompletions = nil
	m.input.Prompt = m.prefix
	m.resetInput()
}

// renderTranscript windows the transcript in its viewport, sized to the
// space left between the history banner and the footer. The viewport keeps
// its scrolled position across content refreshes, unless transcriptToBottom
// is set by new output or a submission, which snaps back to the last line.
// A scrolled view spends one line of the space for the hint about the
// hidden lines. It returns the hint and viewport lines, top to bottom.
func (m *promptModel) renderTranscript(space int) (hintLines, viewportLines []string) {
	if space <= 0 {
		return nil, nil
	}
	vp := &m.transcriptVP
	if m.width <= 0 {
		// the terminal size is still unknown, and the viewport renders
		// nothing without a width: display the lines unwindowed
		return nil, m.transcript
	}
	yOffset := vp.YOffset()
	vp.SetWidth(m.width)
	vp.SetHeight(space)
	// the copy keeps the transcript safe from the in place line splitting
	// the viewport applies on multi line entries
	vp.SetContentLines(append([]string(nil), m.transcript...))
	if m.transcriptToBottom {
		vp.GotoBottom()
		m.transcriptToBottom = false
	} else {
		vp.SetYOffset(yOffset)
	}

	hiddenBelow := vp.TotalLineCount() - vp.VisibleLineCount() - vp.YOffset()
	if hiddenBelow > 0 {
		// one line of the space pays for the hint about the hidden lines
		vp.SetHeight(space - 1)
		vp.SetYOffset(vp.YOffset()) // re-clamp to the reduced space
		hidden := vp.TotalLineCount() - vp.VisibleLineCount()
		hiddenBelow = max(0, hidden-vp.YOffset())
		hintLines = append(hintLines, lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(
			fmt.Sprintf("  ↑ %d/%d lines above (pgup/pgdown, wheel to scroll)",
				hiddenBelow, hidden)))
	}
	return hintLines, strings.Split(vp.View(), "\n")
}

func (m *promptModel) moveHistory(direction int) bool {
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
func (m *promptModel) resetInput() {
	m.input.Reset()
	m.input.SetSuggestions(nil)
	m.historyIndex = -1
	m.historyDraft = ""
	m.updateCompletions()
}

// drainOutput moves the outputs queued by a command, or by its sub prompt
// callback, into the transcript.
func (m *promptModel) drainOutput() {
	if m.takeOutput == nil {
		return
	}
	for _, output := range m.takeOutput() {
		m.appendOutput(output)
	}
}

func (m *promptModel) updateCompletions() {
	if m.completions == nil || strings.TrimSpace(m.input.Value()) == "" {
		m.input.SetSuggestions(nil)
		return
	}
	m.input.SetSuggestions(completionTexts(m.completions(m.input.Value())))
}

func (m *promptModel) appendOutput(output string) {
	output = strings.TrimSuffix(output, "\n")
	if output == "" {
		return
	}
	m.transcript = append(m.transcript, strings.Split(output, "\n")...)
	// new output brings the user back to the bottom of the transcript
	m.transcriptToBottom = true
}

// clearTranscript empties the transcript and resets scrolling, as requested
// by the clear command.
func (m *promptModel) clearTranscript() {
	m.transcript = nil
	m.transcriptToBottom = true
}

func (m *promptModel) View() tea.View {
	// the title, centered at the top of the screen: gonetem, its version
	// and the running project
	title := "gonetem " + options.VERSION
	if m.projectName != "" {
		title += " — Project: " + m.projectName
	}
	titleLine := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Render(
		centerText(title, m.width))

	// a fixed number of lines is reserved above the prompt, for the
	// completion suggestions or, while a command runs, for its waiting
	// message and progress bars: the two are never displayed at the same
	// time, so the prompt line always keeps the same position
	var topLines []string
	if !m.done && m.busy && m.busyMessage != "" {
		style := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11"))
		topLines = append(topLines, style.Render(m.busyMessage))
	}
	if !m.done && m.progress != "" {
		style := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
		for _, line := range strings.Split(m.progress, "\n") {
			topLines = append(topLines, style.Render(line))
		}
	}
	if len(topLines) == 0 && !m.done && m.showMatches && m.completions != nil &&
		strings.TrimSpace(m.input.Value()) != "" {
		topLines = renderSuggestions(m.completions(m.input.Value()))
	}
	if !m.done && len(topLines) < promptSuggestionLines {
		topLines = append(topLines, make([]string, promptSuggestionLines-len(topLines))...)
	}

	var inputLines []string
	if !m.done {
		inputLines = append(inputLines, m.input.View())
	}

	var errorLines []string
	if !m.done && m.promptError != "" {
		for _, line := range strings.Split(m.promptError, "\n") {
			errorLines = append(errorLines, lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(line))
		}
	}

	// the project path, displayed on the last line of the terminal
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

	// the banner delimiting the command history, displayed below the prompt
	label := " History "
	if m.showHeader {
		label = " History — use `exit` to close the project "
	}
	leftDashes, rightDashes := 7, 7
	if labelWidth := lipgloss.Width(label); m.width > labelWidth+14 {
		leftDashes = (m.width - labelWidth) / 2
		rightDashes = m.width - labelWidth - leftDashes
	}
	banner := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(
		strings.Repeat("─", leftDashes) + label + strings.Repeat("─", rightDashes))

	// the history takes the place left on screen between the prompt area
	// and the footer, windowed and scrolled in its viewport
	transcriptSpace := len(m.transcript)
	if m.height > 0 {
		transcriptSpace = max(0, m.height-1-len(topLines)-len(inputLines)-len(errorLines)-
			1-footerLines)
	}
	indicatorLines, transcript := m.renderTranscript(transcriptSpace)

	lines := make([]string, 0, max(m.height, 13))
	lines = append(lines, titleLine)
	lines = append(lines, topLines...)
	lines = append(lines, inputLines...)
	lines = append(lines, errorLines...)
	lines = append(lines, banner)
	lines = append(lines, indicatorLines...)
	lines = append(lines, transcript...)
	if m.height > 0 {
		// the history area fills the screen, keeping the footer on the last
		// line of the terminal
		for len(lines) < m.height-footerLines {
			lines = append(lines, "")
		}
	}
	if footer != "" {
		lines = append(lines, footer)
	}

	// when an alert is displayed, pad all lines to the terminal width before
	// the overlay: bubbleup centers each alert row against the content line
	// below it and, when that line is shorter than the centering offset,
	// pastes the row at column 0, which tears the alert box apart at center
	// positions. uniform line widths keep the box whole.
	if m.alert.HasActiveAlert() && m.width > 0 {
		lines = padLinesWidth(lines, m.width)
	}
	content := strings.Join(lines, "\n")
	view := tea.NewView(m.alert.Render(content))
	view.AltScreen = m.fullscreen
	if m.fullscreen {
		// capture the mouse wheel to scroll the transcript, instead of the
		// terminal native scrollback which is empty in the alt screen
		view.MouseMode = tea.MouseModeCellMotion
	}
	return view
}

// padLinesWidth returns a copy of the lines padded with spaces, or
// truncated when longer, to exactly width cells, preserving their ANSI
// styling. lines overflowing the terminal are truncated rather than left to
// wrap, as the fixed-height layout of View already assumes one screen row
// per line.
func padLinesWidth(lines []string, width int) []string {
	padded := make([]string, len(lines))
	for i, line := range lines {
		switch pad := width - ansi.StringWidth(line); {
		case pad > 0:
			padded[i] = line + strings.Repeat(" ", pad)
		case pad < 0:
			padded[i] = ansi.Truncate(line, width, "")
		default:
			padded[i] = line
		}
	}
	return padded
}

// centerText pads the text with spaces so it is displayed centered in a
// line of the given terminal width; the text is returned unchanged when it
// does not fit or the terminal width is still unknown.
func centerText(text string, width int) string {
	if pad := width - lipgloss.Width(text); pad > 0 {
		return strings.Repeat(" ", pad/2) + text
	}
	return text
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
	for _, match := range matches[:limit] {
		line := "  " + match.Text
		if match.Description != "" {
			line += "  " + match.Description
		}
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(line))
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
