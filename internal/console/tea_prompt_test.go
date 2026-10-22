package console

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/mroy31/gonetem/internal/proto"
)

func TestPromptErrorDoesNotClearInput(t *testing.T) {
	model := newTeaPromptModel("[project]> ", nil, func(string) (bool, error) { return false, nil })
	model.input.SetValue("capture router.0")

	updated, _ := model.Update(promptErrorMsg("capture failed"))
	model = updated.(*teaPromptModel)

	if got := model.input.Value(); got != "capture router.0" {
		t.Fatalf("input after async error = %q, want it preserved", got)
	}
	if !strings.Contains(model.View().Content, "capture failed") {
		t.Fatal("prompt view does not display the asynchronous error")
	}
}

func TestPromptSuggestionsAppearAfterInputAndCompleteNodes(t *testing.T) {
	completer := NewPromptCompleter(&NetemPrompt{
		commands: map[string]*NetemCommand{"start": {}},
		nodes:    []NetemNode{{Name: "router1"}, {Name: "router2"}},
	})
	model := newTeaPromptModel("[project]> ", completer.Complete, func(string) (bool, error) { return false, nil })

	model.updateCompletions()
	if got := model.input.MatchedSuggestions(); len(got) != 0 {
		t.Fatalf("suggestions for empty input = %v, want none", got)
	}
	if strings.Contains(model.View().Content, "start") {
		t.Fatal("completion preview is visible before input")
	}

	model.input.SetValue("start ")
	model.updateCompletions()
	if got := model.input.MatchedSuggestions(); len(got) != 3 || got[0] != "start router1" || got[1] != "start router2" || got[2] != "start all" {
		t.Fatalf("node suggestions after command and space = %v, want router1, router2, all", got)
	}
	if !strings.Contains(model.View().Content, "start router1") {
		t.Fatal("node completion preview is missing after command and space")
	}

	model.input.SetValue("start rou")
	model.input.SetCursor(6)
	model.updateCompletions()
	if got := model.input.MatchedSuggestions(); len(got) != 2 {
		t.Fatalf("node suggestions = %v, want router1 and router2", got)
	}
	if !strings.Contains(model.View().Content, "start router1") {
		t.Fatal("node completion preview is missing")
	}

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	model = updated.(*teaPromptModel)
	if got := model.input.Value(); got != "start router1" {
		t.Fatalf("input after tab completion = %q, want %q", got, "start router1")
	}
	if got, want := model.input.Position(), len([]rune("start router1")); got != want {
		t.Fatalf("cursor after tab completion = %d, want %d", got, want)
	}
}

func TestPromptRetainsSubmittedCommands(t *testing.T) {
	model := newTeaPromptModel("[project]> ", nil, func(string) (bool, error) { return false, nil })
	for _, command := range []string{"status", "help"} {
		model.input.SetValue(command)
		updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		model = updated.(*teaPromptModel)
	}

	view := model.View().Content
	for _, command := range []string{"[project]> status", "[project]> help"} {
		if !strings.Contains(view, command) {
			t.Fatalf("prompt view does not retain command %q", command)
		}
	}
}

func TestPromptClearsSuggestionsAfterSubmit(t *testing.T) {
	completer := func(string) []Completion {
		return []Completion{{Text: "status"}}
	}
	model := newTeaPromptModel("[project]> ", completer, func(string) (bool, error) { return false, nil })
	model.input.SetValue("sta")
	model.updateCompletions()

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(*teaPromptModel)

	if got := model.input.MatchedSuggestions(); len(got) != 0 {
		t.Fatalf("suggestions after submit = %v, want none", got)
	}
	if strings.Contains(model.View().Content, "\n  status") {
		t.Fatal("completion preview is visible after submitting the input")
	}
}

func TestPromptAppendsCommandOutputToTranscript(t *testing.T) {
	model := newTeaPromptModel("[project]> ", nil, func(string) (bool, error) { return false, nil })
	model.appendOutput("Project demo\n- State: Running\n")

	view := model.View().Content
	if !strings.Contains(view, "Project demo\n- State: Running") {
		t.Fatalf("prompt view does not contain command output: %q", view)
	}
}

func TestPromptDisplaysAndClearsProgress(t *testing.T) {
	model := newTeaPromptModel("[project]> ", nil, func(string) (bool, error) { return false, nil })

	updated, _ := model.Update(promptProgressMsg(progressText("Save nodes:", 1, 2)))
	model = updated.(*teaPromptModel)
	view := model.View().Content
	if !strings.Contains(view, "Save nodes:") || !strings.Contains(view, "1/2") {
		t.Fatalf("prompt view does not display the save progress: %q", view)
	}

	updated, _ = model.Update(promptOutputMsg("Project saved to /work/demo.gnet"))
	model = updated.(*teaPromptModel)

	updated, _ = model.Update(promptProgressMsg(""))
	model = updated.(*teaPromptModel)
	view = model.View().Content
	if strings.Contains(view, "Save nodes:") {
		t.Fatalf("progress line is still visible after being cleared: %q", view)
	}
	if !strings.Contains(view, "Project saved to /work/demo.gnet") {
		t.Fatalf("async output message is not retained in the transcript: %q", view)
	}
}

func TestProgressTextRendersBarAndPendingState(t *testing.T) {
	if got, want := progressText("Save nodes:", 0, 0), "Save nodes: ..."; got != want {
		t.Fatalf("pending progress = %q, want %q", got, want)
	}

	halfBar := strings.Repeat("█", progressBarWidth/2) + strings.Repeat("░", progressBarWidth/2)
	got, want := progressText("Save nodes:", 5, 10), "Save nodes: ["+halfBar+"] 5/10"
	if got != want {
		t.Fatalf("half progress = %q, want %q", got, want)
	}

	fullBar := strings.Repeat("█", progressBarWidth)
	got, want = progressText("Save nodes:", 10, 10), "Save nodes: ["+fullBar+"] 10/10"
	if got != want {
		t.Fatalf("full progress = %q, want %q", got, want)
	}
}

func TestRunProgressRendersOnlyRunningPhases(t *testing.T) {
	progress := newRunProgress()

	// increments of a phase not yet announced are ignored, as the mpb bars
	// do not exist yet either
	if got := progress.handle(&proto.TopologyRunMsg{Code: proto.TopologyRunMsg_NODE_START}); got != "" {
		t.Fatalf("progress before any count = %q, want empty", got)
	}

	// the node count announces the start and configuration phases
	got := progress.handle(&proto.TopologyRunMsg{Code: proto.TopologyRunMsg_NODE_COUNT, Total: 2})
	if !strings.Contains(got, "Node - Start:") || !strings.Contains(got, "Node - Configuration:") {
		t.Fatalf("node phases not displayed after node count: %q", got)
	}
	if strings.Contains(got, "Link - Create:") {
		t.Fatalf("link phase displayed before its count: %q", got)
	}

	// each increment moves the matching phase counter forward
	got = progress.handle(&proto.TopologyRunMsg{Code: proto.TopologyRunMsg_NODE_START})
	halfBar := strings.Repeat("█", progressBarWidth/2) + strings.Repeat("░", progressBarWidth/2)
	if !strings.Contains(got, "Node - Start: ["+halfBar+"] 1/2") {
		t.Fatalf("node start progress not updated: %q", got)
	}

	// the link phase joins the display when announced
	got = progress.handle(&proto.TopologyRunMsg{Code: proto.TopologyRunMsg_LINK_COUNT, Total: 1})
	if !strings.Contains(got, "Link - Create:") {
		t.Fatalf("link phase not displayed after link count: %q", got)
	}

	// a completed phase disappears, like the mpb bar removed on completion
	progress.handle(&proto.TopologyRunMsg{Code: proto.TopologyRunMsg_NODE_START})
	got = progress.handle(&proto.TopologyRunMsg{Code: proto.TopologyRunMsg_NODE_LOADCONFIG})
	if strings.Contains(got, "Node - Start:") {
		t.Fatalf("completed node start phase still displayed: %q", got)
	}

	// node messages force-complete all phases, as on the mpb path
	got = progress.handle(&proto.TopologyRunMsg{
		Code:         proto.TopologyRunMsg_NODE_MESSAGES,
		NodeMessages: []*proto.TopologyRunMsg_NodeMessages{{Name: "r1", Messages: []string{"a warning"}}},
	})
	if got != "" {
		t.Fatalf("progress not cleared after node messages: %q", got)
	}

	// and the warnings are rendered for the transcript
	output := nodeMessagesOutput(&proto.TopologyRunMsg{
		NodeMessages: []*proto.TopologyRunMsg_NodeMessages{{Name: "r1", Messages: []string{"a warning", ""}}},
	})
	if !strings.Contains(output, "r1:") || !strings.Contains(output, "a warning") {
		t.Fatalf("node messages output misses content: %q", output)
	}
}

func TestPromptScrollsTranscriptWithPgUpPgDownAndWheel(t *testing.T) {
	model := newTeaPromptModel("[project]> ", nil, func(string) (bool, error) { return false, nil })
	model.fullscreen = true
	for i := 1; i <= 30; i++ {
		model.transcript = append(model.transcript, fmt.Sprintf("output line %d", i))
	}

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	model = updated.(*teaPromptModel)

	// at the bottom, the tail of the transcript is visible, the head is not
	view := model.View().Content
	if !strings.Contains(view, "output line 30\n") {
		t.Fatalf("tail of the transcript is not visible: %q", view)
	}
	if strings.Contains(view, "output line 1\n") {
		t.Fatalf("head of the transcript is visible without scrolling: %q", view)
	}

	// pgup scrolls back through the older lines
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	model = updated.(*teaPromptModel)
	view = model.View().Content
	if !strings.Contains(view, "output line 1\n") {
		t.Fatalf("pgup does not show the head of the transcript: %q", view)
	}
	if strings.Contains(view, "output line 30\n") {
		t.Fatalf("pgup still shows the bottom of the transcript: %q", view)
	}
	if !strings.Contains(view, "lines above") {
		t.Fatalf("scroll indicator is missing when scrolled back: %q", view)
	}

	// the mouse wheel scrolls by a few lines
	updated, _ = model.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	model = updated.(*teaPromptModel)
	if model.scrollOffset == 0 {
		t.Fatal("mouse wheel down did not move the scroll offset back to the bottom")
	}

	// pgdown returns to the bottom
	for range 3 {
		updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
		model = updated.(*teaPromptModel)
	}
	view = model.View().Content
	if !strings.Contains(view, "output line 30\n") {
		t.Fatalf("pgdown does not return to the bottom of the transcript: %q", view)
	}
	if strings.Contains(view, "lines above") {
		t.Fatalf("scroll indicator is still visible at the bottom: %q", view)
	}
}

func TestPromptNewOutputResetsScrollToBottom(t *testing.T) {
	model := newTeaPromptModel("[project]> ", nil, func(string) (bool, error) { return false, nil })
	model.fullscreen = true
	for i := 1; i <= 30; i++ {
		model.transcript = append(model.transcript, fmt.Sprintf("output line %d", i))
	}

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	model = updated.(*teaPromptModel)
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	model = updated.(*teaPromptModel)

	model.appendOutput("fresh command output\n")

	view := model.View().Content
	if model.scrollOffset != 0 {
		t.Fatalf("scroll offset after new output = %d, want 0", model.scrollOffset)
	}
	if !strings.Contains(view, "fresh command output") || !strings.Contains(view, "output line 30\n") {
		t.Fatalf("new output does not bring back the bottom of the transcript: %q", view)
	}
}

func TestPromptRunsQueuedEditorCommandAfterSubmit(t *testing.T) {
	editorRan := false
	queued := false
	var model *teaPromptModel
	model = newTeaPromptModel("[project]> ", nil, func(string) (bool, error) {
		// simulate the edit command handing an external editor over to the TUI
		if !queued {
			queued = true
			model.pendingExec = func() tea.Msg {
				editorRan = true
				return nil
			}
		}
		return false, nil
	})

	model.input.SetValue("edit")
	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(*teaPromptModel)

	if cmd == nil {
		t.Fatal("no command returned to run the queued editor")
	}
	if model.pendingExec != nil {
		t.Fatal("pending editor command was not consumed by the submission")
	}
	cmd()
	if !editorRan {
		t.Fatal("the queued editor command was not the one returned")
	}

	// next submission without a queued command returns none
	updated, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(*teaPromptModel)
	if cmd != nil {
		t.Fatal("a command was returned while no editor is pending")
	}
}

func TestPromptEditorFinishReportsErrorBelowPrompt(t *testing.T) {
	var queued []string
	var model *teaPromptModel
	model = newTeaPromptModel("[project]> ", nil, func(string) (bool, error) { return false, nil })
	model.takeOutput = func() []string {
		output := queued
		queued = nil
		return output
	}

	// successful editor exit: finish consumes the file and its output lands
	// in the transcript
	updated, _ := model.Update(editorFinishedMsg{err: nil, finish: func(editorErr error) error {
		queued = append(queued, "Network file has been updated")
		return nil
	}})
	model = updated.(*teaPromptModel)
	if !strings.Contains(model.View().Content, "Network file has been updated") {
		t.Fatalf("editor result is not displayed in the transcript: %q", model.View().Content)
	}

	// failing finish: the error is displayed below the prompt
	updated, _ = model.Update(editorFinishedMsg{err: nil, finish: func(editorErr error) error {
		return errors.New("Unable to write modified network file on server")
	}})
	model = updated.(*teaPromptModel)
	if !strings.Contains(model.View().Content, "Unable to write modified network file on server") {
		t.Fatalf("finish error is not displayed: %q", model.View().Content)
	}

	// editor error itself is passed to finish
	var gotEditorErr error
	updated, _ = model.Update(editorFinishedMsg{err: errors.New("editor killed"), finish: func(editorErr error) error {
		gotEditorErr = editorErr
		return nil
	}})
	model = updated.(*teaPromptModel)
	if gotEditorErr == nil || gotEditorErr.Error() != "editor killed" {
		t.Fatalf("finish received editor error %v, want %q", gotEditorErr, "editor killed")
	}
}

func TestClearCommandRequestsTranscriptClear(t *testing.T) {
	p := &NetemPrompt{commands: make(map[string]*NetemCommand)}
	p.RegisterCommands()

	if err := p.Execute("clear"); err != nil {
		t.Fatalf("clear command failed: %v", err)
	}
	if !p.takePendingClear() {
		t.Fatal("clear did not request a transcript clear")
	}
	if p.takePendingClear() {
		t.Fatal("transcript clear request was not consumed")
	}
}

func TestPromptClearTranscriptResetsScroll(t *testing.T) {
	model := newTeaPromptModel("[project]> ", nil, func(string) (bool, error) { return false, nil })
	model.appendOutput("first output\nsecond output")
	model.scrollBy(3)

	// the submitted clear line is echoed to the transcript before the
	// command runs, and is wiped out with the rest by the clear
	model.input.SetValue("clear")
	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(*teaPromptModel)
	if len(model.transcript) == 0 {
		t.Fatal("submitted command was not echoed to the transcript")
	}

	model.clearTranscript()

	if len(model.transcript) != 0 {
		t.Fatalf("transcript not cleared: %v", model.transcript)
	}
	if model.scrollOffset != 0 {
		t.Fatalf("scroll offset after clear = %d, want 0", model.scrollOffset)
	}
	if view := model.View().Content; strings.Contains(view, "output") {
		t.Fatalf("cleared transcript is still rendered: %q", view)
	}
}

func TestPromptSubPromptListsFilesAndDisplaysContent(t *testing.T) {
	var queued []string
	var selected string
	model := newTeaPromptModel("[project]> ", nil, func(string) (bool, error) { return false, nil })
	model.takeOutput = func() []string {
		output := queued
		queued = nil
		return output
	}
	model.startSubPrompt(&inputPromptRequest{
		prefix:      "Select configuration file: ",
		completions: []Completion{{Text: "router.frr.conf"}, {Text: "router.host.conf"}},
		submit: func(name string) {
			selected = name
			queued = append(queued, "#####  "+name+"  #####\nrouter-id 1.1.1.1\n")
		},
	})

	model.input.SetValue("router")
	model.updateCompletions()
	view := model.View().Content
	if !strings.Contains(view, "Select configuration file:") {
		t.Fatalf("sub prompt prefix is missing: %q", view)
	}
	if !strings.Contains(view, "router.frr.conf") {
		t.Fatalf("sub prompt does not list available config files: %q", view)
	}

	model.input.SetValue("router.frr.conf")
	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(*teaPromptModel)

	if selected != "router.frr.conf" {
		t.Fatalf("selected config file = %q, want %q", selected, "router.frr.conf")
	}
	view = model.View().Content
	if !strings.Contains(view, "Select configuration file: router.frr.conf") {
		t.Fatalf("sub prompt answer is not retained in the transcript: %q", view)
	}
	if !strings.Contains(view, "router-id 1.1.1.1") {
		t.Fatalf("configuration file content is not displayed: %q", view)
	}
	if model.subPrompt != nil {
		t.Fatal("sub prompt is still active after submitting an answer")
	}
	if !strings.Contains(view, "[project]> ") {
		t.Fatalf("main prompt prefix is not restored: %q", view)
	}
}

func TestPromptSubPromptCancelWithEscapeKeepsMainPrompt(t *testing.T) {
	submitted := false
	model := newTeaPromptModel("[project]> ", nil, func(string) (bool, error) { return false, nil })
	model.startSubPrompt(&inputPromptRequest{
		prefix: "Select configuration file: ",
		submit: func(string) { submitted = true },
	})

	model.input.SetValue("router")
	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	model = updated.(*teaPromptModel)

	if submitted {
		t.Fatal("cancelled sub prompt must not submit the answer")
	}
	if model.subPrompt != nil {
		t.Fatal("sub prompt is still active after escape")
	}
	if model.done {
		t.Fatal("escape on sub prompt must not quit the main prompt")
	}
	view := model.View().Content
	if strings.Contains(view, "Select configuration file:") {
		t.Fatalf("sub prompt is still visible after escape: %q", view)
	}
	if !strings.Contains(view, "[project]> ") {
		t.Fatalf("main prompt prefix is not restored: %q", view)
	}
}

func TestProjectPromptUsesFullscreenAndErrorBelowPrompt(t *testing.T) {
	model := newTeaPromptModel("[demo]> ", nil, func(string) (bool, error) { return false, nil })
	model.projectName = "demo"
	model.projectPath = "/work/demo.gnet"
	model.showHeader = true
	model.fullscreen = true
	model.transcript = []string{"Previous output"}

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	model = updated.(*teaPromptModel)
	updated, _ = model.Update(promptErrorMsg("command failed"))
	model = updated.(*teaPromptModel)

	view := model.View()
	if !view.AltScreen {
		t.Fatal("project prompt does not use the alternate fullscreen buffer")
	}
	lines := strings.Split(view.Content, "\n")
	if len(lines) != 12 {
		t.Fatalf("prompt rendered %d lines, want terminal height 12", len(lines))
	}
	for _, text := range []string{
		"Welcome to gonetem console",
		"Project: demo",
		"Path: /work/demo.gnet",
		"[demo]> ",
	} {
		if !strings.Contains(view.Content, text) {
			t.Fatalf("project prompt does not display %q", text)
		}
	}
	promptIndex := -1
	for i, line := range lines {
		if strings.Contains(line, "[demo]> ") {
			promptIndex = i
			break
		}
	}
	if promptIndex == -1 {
		t.Fatalf("prompt input line is missing: %q", view.Content)
	}
	if !strings.Contains(lines[promptIndex+1], "command failed") {
		t.Fatalf("the error is not displayed just below the prompt: %q", lines[promptIndex+1])
	}
}

func TestPromptDisplaysCommandErrorBelowPromptUntilNextSubmit(t *testing.T) {
	failing := true
	var model *teaPromptModel
	model = newTeaPromptModel("[project]> ", nil, func(string) (bool, error) {
		if failing {
			return false, errors.New("node router1 is not running")
		}
		return false, nil
	})

	model.input.SetValue("start router1")
	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(*teaPromptModel)

	lines := strings.Split(model.View().Content, "\n")
	promptIndex := -1
	for i, line := range lines {
		if strings.Contains(line, "[project]> ") {
			promptIndex = i
		}
	}
	if promptIndex == -1 || promptIndex+1 >= len(lines) {
		t.Fatalf("prompt input line is missing: %q", model.View().Content)
	}
	if !strings.Contains(lines[promptIndex+1], "node router1 is not running") {
		t.Fatalf("returned error is not displayed just below the prompt: %q", model.View().Content)
	}

	failing = false
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(*teaPromptModel)

	if strings.Contains(model.View().Content, "node router1 is not running") {
		t.Fatalf("error is still displayed after a new submission: %q", model.View().Content)
	}
}
