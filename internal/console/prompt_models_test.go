package console

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mroy31/gonetem/internal/options"
	"github.com/mroy31/gonetem/internal/proto"
	"go.dalton.dog/bubbleup/v2"
	"google.golang.org/grpc"
)

func TestPromptErrorDoesNotClearInput(t *testing.T) {
	model := newPromptModel("[project]> ", nil, func(string) (bool, error) { return false, nil })
	model.input.SetValue("capture router.0")

	updated, cmd := model.Update(promptAlertMsg{
		alertType: bubbleup.ErrorKey,
		alertMsg:  "capture failed",
	})
	model = updated.(*promptModel)
	if cmd == nil {
		t.Fatal("no alert command returned for the asynchronous error")
	}
	updated, _ = model.Update(cmd())
	model = updated.(*promptModel)

	if got := model.input.Value(); got != "capture router.0" {
		t.Fatalf("input after async error = %q, want it preserved", got)
	}
	if !strings.Contains(model.View().Content, "capture failed") {
		t.Fatal("prompt view does not display the asynchronous error as an alert")
	}
}

func TestPromptSuggestionsAppearAfterInputAndCompleteNodes(t *testing.T) {
	completer := NewPromptCompleter(&NetemPrompt{
		commands: map[string]*NetemCommand{"start": {}},
		nodes:    []NetemNode{{Name: "router1"}, {Name: "router2"}},
	})
	model := newPromptModel("[project]> ", completer.Complete, func(string) (bool, error) { return false, nil })

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
	model = updated.(*promptModel)
	if got := model.input.Value(); got != "start router1" {
		t.Fatalf("input after tab completion = %q, want %q", got, "start router1")
	}
	if got, want := model.input.Position(), len([]rune("start router1")); got != want {
		t.Fatalf("cursor after tab completion = %d, want %d", got, want)
	}
}

func TestPromptRetainsSubmittedCommands(t *testing.T) {
	model := newPromptModel("[project]> ", nil, func(string) (bool, error) { return false, nil })
	for _, command := range []string{"status", "help"} {
		model.input.SetValue(command)
		updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		model = updated.(*promptModel)
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
	model := newPromptModel("[project]> ", completer, func(string) (bool, error) { return false, nil })
	model.input.SetValue("sta")
	model.updateCompletions()

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(*promptModel)

	if got := model.input.MatchedSuggestions(); len(got) != 0 {
		t.Fatalf("suggestions after submit = %v, want none", got)
	}
	if strings.Contains(model.View().Content, "\n  status") {
		t.Fatal("completion preview is visible after submitting the input")
	}
}

func TestPromptAppendsCommandOutputToTranscript(t *testing.T) {
	model := newPromptModel("[project]> ", nil, func(string) (bool, error) { return false, nil })
	model.appendOutput("Project demo\n- State: Running\n")

	view := model.View().Content
	if !strings.Contains(view, "Project demo\n- State: Running") {
		t.Fatalf("prompt view does not contain command output: %q", view)
	}
}

func TestPromptDisplaysAndClearsProgress(t *testing.T) {
	model := newPromptModel("[project]> ", nil, func(string) (bool, error) { return false, nil })

	updated, _ := model.Update(promptProgressMsg(progressText("Save nodes:", 1, 2)))
	model = updated.(*promptModel)
	view := model.View().Content
	if !strings.Contains(view, "Save nodes:") || !strings.Contains(view, "1/2") {
		t.Fatalf("prompt view does not display the save progress: %q", view)
	}

	updated, cmd := model.Update(promptAlertMsg{
		alertType: bubbleup.InfoKey,
		alertMsg:  "Project saved to /work/demo.gnet",
	})
	model = updated.(*promptModel)
	// the output is displayed as a transient alert: run the returned
	// command, as the program does, to activate the alert
	if cmd == nil {
		t.Fatal("no alert command returned for the output message")
	}
	updated, _ = model.Update(cmd())
	model = updated.(*promptModel)

	updated, _ = model.Update(promptProgressMsg(""))
	model = updated.(*promptModel)
	view = model.View().Content
	if strings.Contains(view, "Save nodes:") {
		t.Fatalf("progress line is still visible after being cleared: %q", view)
	}
	if !strings.Contains(view, "Project saved to /work/demo.gnet") {
		t.Fatalf("async output message is not displayed as an alert: %q", view)
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

func TestRunProgressReloadTracksCloseThenRunPhases(t *testing.T) {
	progress := newReloadProgress()

	// the reload stream first announces the close phase counts
	got := progress.handle(&proto.TopologyRunMsg{Code: proto.TopologyRunMsg_NODE_COUNT, Total: 2})
	got = progress.handle(&proto.TopologyRunMsg{Code: proto.TopologyRunMsg_BRIDGE_COUNT, Total: 1})
	if !strings.Contains(got, "Node - Close:") || !strings.Contains(got, "Bridge - Close:") {
		t.Fatalf("close phases not displayed after close counts: %q", got)
	}
	if strings.Contains(got, "Node - Start:") {
		t.Fatalf("run phases displayed during the close phase: %q", got)
	}

	// the close increments complete and clear the close bars
	got = progress.handle(&proto.TopologyRunMsg{Code: proto.TopologyRunMsg_NODE_STOP})
	got = progress.handle(&proto.TopologyRunMsg{Code: proto.TopologyRunMsg_NODE_STOP})
	got = progress.handle(&proto.TopologyRunMsg{Code: proto.TopologyRunMsg_NODE_RM})
	if got != "" {
		t.Fatalf("close phases not completed: %q", got)
	}

	// the run phase then announces itself and displays its own bars
	got = progress.handle(&proto.TopologyRunMsg{Code: proto.TopologyRunMsg_NODE_COUNT, Total: 2})
	got = progress.handle(&proto.TopologyRunMsg{Code: proto.TopologyRunMsg_BRIDGE_COUNT, Total: 1})
	got = progress.handle(&proto.TopologyRunMsg{Code: proto.TopologyRunMsg_LINK_COUNT, Total: 1})
	if !strings.Contains(got, "Node - Start:") || !strings.Contains(got, "Node - Configuration:") ||
		!strings.Contains(got, "Link - Create:") || !strings.Contains(got, "Bridge - Create:") {
		t.Fatalf("run phases not displayed after run counts: %q", got)
	}
	if strings.Contains(got, "Close") {
		t.Fatalf("close phases displayed during the run phase: %q", got)
	}

	// run increments move the run counters, not the close ones
	got = progress.handle(&proto.TopologyRunMsg{Code: proto.TopologyRunMsg_NODE_START})
	halfBar := strings.Repeat("█", progressBarWidth/2) + strings.Repeat("░", progressBarWidth/2)
	if !strings.Contains(got, "Node - Start: ["+halfBar+"] 1/2") {
		t.Fatalf("run progress not updated: %q", got)
	}

	// node messages still force-complete everything
	got = progress.handle(&proto.TopologyRunMsg{Code: proto.TopologyRunMsg_NODE_MESSAGES})
	if got != "" {
		t.Fatalf("progress not cleared after node messages: %q", got)
	}
}

func TestCloseProgressTracksNodeAndBridgePhases(t *testing.T) {
	progress := newCloseProgress()

	// increments of a phase not yet announced are ignored
	if got := progress.handle(&proto.ProjectCloseMsg{Code: proto.ProjectCloseMsg_NODE_CLOSE}); got != "" {
		t.Fatalf("progress before any count = %q, want empty", got)
	}

	got := progress.handle(&proto.ProjectCloseMsg{Code: proto.ProjectCloseMsg_NODE_COUNT, Total: 2})
	got = progress.handle(&proto.ProjectCloseMsg{Code: proto.ProjectCloseMsg_BRIDGE_COUNT, Total: 1})
	if !strings.Contains(got, "Node - Close:") || !strings.Contains(got, "Bridge - Close:") {
		t.Fatalf("close phases not displayed after counts: %q", got)
	}

	got = progress.handle(&proto.ProjectCloseMsg{Code: proto.ProjectCloseMsg_NODE_CLOSE})
	halfBar := strings.Repeat("█", progressBarWidth/2) + strings.Repeat("░", progressBarWidth/2)
	if !strings.Contains(got, "Node - Close: ["+halfBar+"] 1/2") {
		t.Fatalf("node close progress not updated: %q", got)
	}

	// completed phases disappear, and the display empties once all are done
	got = progress.handle(&proto.ProjectCloseMsg{Code: proto.ProjectCloseMsg_NODE_CLOSE})
	got = progress.handle(&proto.ProjectCloseMsg{Code: proto.ProjectCloseMsg_BRIDGE_CLOSE})
	if got != "" {
		t.Fatalf("close phases not completed: %q", got)
	}
}

func TestPromptQuitsWhenProjectCloseFinished(t *testing.T) {
	model := newPromptModel("[demo]> ", nil, func(string) (bool, error) { return false, nil })

	// the exit command keeps the TUI on screen while it closes the project,
	// only the message sent once its stream completed lets it leave
	updated, cmd := model.Update(promptQuitMsg{})
	model = updated.(*promptModel)
	if !model.done {
		t.Fatal("the prompt is not done after the quit message")
	}
	if cmd == nil {
		t.Fatal("the quit message does not return a command")
	}
	if _, isQuit := cmd().(tea.QuitMsg); !isQuit {
		t.Fatal("the returned command does not quit the program")
	}
}

func TestPromptScrollsTranscriptWithPgUpPgDownAndWheel(t *testing.T) {
	model := newPromptModel("[project]> ", nil, func(string) (bool, error) { return false, nil })
	model.fullscreen = true
	for i := 1; i <= 30; i++ {
		model.transcript = append(model.transcript, fmt.Sprintf("output line %d", i))
	}

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	model = updated.(*promptModel)

	// at the bottom, the tail of the transcript is visible, the head is not
	// (exact line match, trimmed of the viewport width padding: line 1
	// would otherwise match line 10 or line 19)
	visible := func(view, line string) bool {
		for _, text := range strings.Split(view, "\n") {
			if strings.TrimRight(text, " ") == line {
				return true
			}
		}
		return false
	}
	view := model.View().Content
	if !visible(view, "output line 30") {
		t.Fatalf("tail of the transcript is not visible: %q", view)
	}
	if visible(view, "output line 1") {
		t.Fatalf("head of the transcript is visible without scrolling: %q", view)
	}

	// pgup scrolls back through the older lines, a page at a time
	for range 3 {
		updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
		model = updated.(*promptModel)
	}
	view = model.View().Content
	if !visible(view, "output line 1") {
		t.Fatalf("pgup does not show the head of the transcript: %q", view)
	}
	if visible(view, "output line 30") {
		t.Fatalf("pgup still shows the bottom of the transcript: %q", view)
	}
	if !strings.Contains(view, "lines above") {
		t.Fatalf("scroll indicator is missing when scrolled back: %q", view)
	}

	// the mouse wheel scrolls by a few lines
	updated, _ = model.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	model = updated.(*promptModel)
	if model.transcriptVP.YOffset() == 0 {
		t.Fatal("mouse wheel down did not move the scroll offset back to the bottom")
	}

	// pgdown returns to the bottom
	for range 3 {
		updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
		model = updated.(*promptModel)
	}
	view = model.View().Content
	if !visible(view, "output line 30") {
		t.Fatalf("pgdown does not return to the bottom of the transcript: %q", view)
	}
	if strings.Contains(view, "lines above") {
		t.Fatalf("scroll indicator is still visible at the bottom: %q", view)
	}
}

func TestPromptBusyFreezesInputButKeepsScrolling(t *testing.T) {
	submitted := false
	model := newPromptModel("[project]> ", nil, func(string) (bool, error) {
		submitted = true
		return false, nil
	})
	model.fullscreen = true
	for i := 1; i <= 30; i++ {
		model.transcript = append(model.transcript, fmt.Sprintf("output line %d", i))
	}

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	model = updated.(*promptModel)

	// the busy state freezes typing and submitting
	waiting := "Reloading the project, please wait until it completes..."
	updated, _ = model.Update(promptBusyMsg{busy: true, message: waiting})
	model = updated.(*promptModel)
	if !model.busy {
		t.Fatal("promptBusyMsg did not freeze the prompt")
	}

	// the waiting message is shown above the progress bars
	model.Update(promptProgressMsg("Node - Close: [====>    ] 1/4"))
	view := model.View().Content
	waitAt, barAt := strings.Index(view, waiting), strings.Index(view, "Node - Close:")
	if waitAt < 0 {
		t.Fatalf("waiting message is not displayed while busy: %q", view)
	}
	if barAt < 0 || waitAt > barAt {
		t.Fatalf("waiting message is not above the progress bars: %q", view)
	}

	updated, _ = model.Update(tea.KeyPressMsg{Code: 'r', Text: "reload"})
	model = updated.(*promptModel)
	if model.input.Value() != "" {
		t.Fatalf("typing is not frozen while busy: %q", model.input.Value())
	}
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(*promptModel)
	if submitted {
		t.Fatal("enter must not submit a command while busy")
	}

	// transcript scrolling keeps working, and output keeps arriving
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	model = updated.(*promptModel)
	if model.transcriptVP.YOffset() == 0 {
		t.Fatal("pgup must keep scrolling while busy")
	}
	updated, cmd := model.Update(promptAlertMsg{
		alertType: bubbleup.WarnKey,
		alertMsg:  "warning from the running command",
	})
	model = updated.(*promptModel)
	if cmd == nil {
		t.Fatal("no alert command returned for the output while busy")
	}
	updated, _ = model.Update(cmd())
	model = updated.(*promptModel)
	if !model.alert.HasActiveAlert() {
		t.Fatal("command output while busy does not display an alert")
	}

	// leaving busy restores typing and submitting, and clears the message
	updated, _ = model.Update(promptBusyMsg{})
	model = updated.(*promptModel)
	if model.busy {
		t.Fatal("releasing promptBusyMsg did not release the prompt")
	}
	if strings.Contains(model.View().Content, waiting) {
		t.Fatal("waiting message is still displayed after the command completed")
	}
	updated, _ = model.Update(tea.KeyPressMsg{Code: 's', Text: "status"})
	model = updated.(*promptModel)
	if model.input.Value() != "status" {
		t.Fatalf("typing is still frozen after busy: %q", model.input.Value())
	}
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !submitted {
		t.Fatal("enter did not submit the command after busy released")
	}
}

func TestPromptNewOutputResetsScrollToBottom(t *testing.T) {
	model := newPromptModel("[project]> ", nil, func(string) (bool, error) { return false, nil })
	model.fullscreen = true
	for i := 1; i <= 30; i++ {
		model.transcript = append(model.transcript, fmt.Sprintf("output line %d", i))
	}

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	model = updated.(*promptModel)
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	model = updated.(*promptModel)

	model.appendOutput("fresh command output\n")

	view := model.View().Content
	if !model.transcriptVP.AtBottom() {
		t.Fatalf("viewport is not back at the bottom after new output: yOffset %d",
			model.transcriptVP.YOffset())
	}
	if !strings.Contains(view, "fresh command output") || !strings.Contains(view, "output line 30") {
		t.Fatalf("new output does not bring back the bottom of the transcript: %q", view)
	}
}

func TestPromptRunsQueuedEditorCommandAfterSubmit(t *testing.T) {
	editorRan := false
	queued := false
	var model *promptModel
	model = newPromptModel("[project]> ", nil, func(string) (bool, error) {
		// simulate the edit command handing an external editor over to the TUI
		if !queued {
			queued = true
			model.pendingCmd = func() tea.Msg {
				editorRan = true
				return nil
			}
		}
		return false, nil
	})

	model.input.SetValue("edit")
	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(*promptModel)

	if cmd == nil {
		t.Fatal("no command returned to run the queued editor")
	}
	if model.pendingCmd != nil {
		t.Fatal("pending command was not consumed by the submission")
	}
	cmd()
	if !editorRan {
		t.Fatal("the queued editor command was not the one returned")
	}

	// next submission without a queued command returns none
	updated, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(*promptModel)
	if cmd != nil {
		t.Fatal("a command was returned while no command is pending")
	}
}

func TestPromptEditorFinishReportsErrorAsAlert(t *testing.T) {
	var queued []string
	var model *promptModel
	model = newPromptModel("[project]> ", nil, func(string) (bool, error) { return false, nil })
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
	model = updated.(*promptModel)
	if !strings.Contains(model.View().Content, "Network file has been updated") {
		t.Fatalf("editor result is not displayed in the transcript: %q", model.View().Content)
	}

	// failing finish: the error is displayed as an alert
	updated, alertCmd := model.Update(editorFinishedMsg{err: nil, finish: func(editorErr error) error {
		return errors.New("Unable to write modified network file on server")
	}})
	model = updated.(*promptModel)
	if alertCmd == nil {
		t.Fatal("no alert command returned for the finish error")
	}
	updated, _ = model.Update(alertCmd())
	model = updated.(*promptModel)
	if !strings.Contains(model.View().Content, "Unable to write modified network file on server") {
		t.Fatalf("finish error is not displayed as an alert: %q", model.View().Content)
	}

	// editor error itself is passed to finish
	var gotEditorErr error
	updated, _ = model.Update(editorFinishedMsg{err: errors.New("editor killed"), finish: func(editorErr error) error {
		gotEditorErr = editorErr
		return nil
	}})
	model = updated.(*promptModel)
	if gotEditorErr == nil || gotEditorErr.Error() != "editor killed" {
		t.Fatalf("finish received editor error %v, want %q", gotEditorErr, "editor killed")
	}
}

func TestClearCommandQueuesClearMessage(t *testing.T) {
	p := &NetemPrompt{commands: make(map[string]*NetemCommand)}
	p.RegisterCommands()

	if err := p.Execute("clear"); err != nil {
		t.Fatalf("clear command failed: %v", err)
	}
	cmd := p.takePendingCmd()
	if cmd == nil {
		t.Fatal("clear did not queue a transcript clear command")
	}
	if _, ok := cmd().(promptClearMsg); !ok {
		t.Fatal("the queued command does not send a promptClearMsg")
	}
	if p.takePendingCmd() != nil {
		t.Fatal("transcript clear command was not consumed")
	}
}

func TestTopoGraphCommandRegisteredAndCompleted(t *testing.T) {
	p := &NetemPrompt{commands: make(map[string]*NetemCommand)}
	p.RegisterCommands()

	cmd, ok := p.commands["topoGraph"]
	if !ok {
		t.Fatal("topoGraph command not registered")
	}
	if cmd.Usage != "topoGraph" || len(cmd.Args) != 0 {
		t.Fatalf("unexpected topoGraph command shape: %+v", cmd)
	}
	if _, legacy := p.commands["topo"]; legacy {
		t.Fatal("legacy topo command still registered")
	}
	found := false
	for _, c := range NewPromptCompleter(p).Complete("topo") {
		if strings.Contains(c.Text, "topoGraph") {
			found = true
		}
	}
	if !found {
		t.Fatal("completer does not offer topoGraph")
	}
}

// fakeTopologyCheckClient is a server client stub only implementing the
// TopologyCheck call used by the check command.
type fakeTopologyCheckClient struct {
	proto.NetemClient
	ack *proto.AckResponse
}

func (f *fakeTopologyCheckClient) TopologyCheck(
	ctx context.Context, in *proto.ProjectRequest, opts ...grpc.CallOption,
) (*proto.AckResponse, error) {
	return f.ack, nil
}

func TestCheckCommandQueuesInfoAlertNotModal(t *testing.T) {
	p := &NetemPrompt{prjID: "demo"}

	client := &fakeTopologyCheckClient{ack: &proto.AckResponse{
		Status: &proto.Status{Code: proto.StatusCode_OK},
	}}
	if err := p.Check(client, nil); err != nil {
		t.Fatalf("check command failed: %v", err)
	}
	if outputs := p.takePromptOutput(); len(outputs) != 0 {
		t.Fatalf("check queued a modal output instead of an alert: %q", outputs)
	}
	cmd := p.takePendingCmd()
	if cmd == nil {
		t.Fatal("check did not queue an alert command")
	}
	msg, ok := cmd().(promptAlertMsg)
	if !ok {
		t.Fatalf("the queued command does not send a promptAlertMsg: %T", msg)
	}
	if msg.alertType != bubbleup.InfoKey || msg.alertMsg != "Network is OK" {
		t.Fatalf("unexpected alert: %s %q", msg.alertType, msg.alertMsg)
	}

	// a topology reported in error returns an error instead, displayed as an
	// error alert by the prompt submission
	failing := &fakeTopologyCheckClient{ack: &proto.AckResponse{
		Status: &proto.Status{Code: proto.StatusCode_ERROR, Error: "link r1-r2 is invalid"},
	}}
	err := p.Check(failing, nil)
	if err == nil || err.Error() != "link r1-r2 is invalid" {
		t.Fatalf("check on a broken topology returned %v, want the topology error", err)
	}
}

func TestPromptRunsStartCmdOnStartup(t *testing.T) {
	model := newPromptModel("[demo]> ", nil, func(string) (bool, error) { return false, nil })
	// like NewPrompt starting the topology of an opened project
	model.startCmd = func() tea.Msg {
		return promptAlertMsg{alertType: bubbleup.ErrorKey, alertMsg: "topology start failed"}
	}

	cmd := model.Init()
	if cmd == nil {
		t.Fatal("Init does not run the start command")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("Init does not batch the input focus and the start command: %T", batch)
	}
	var startMsg tea.Msg
	for _, c := range batch {
		if c == nil {
			continue
		}
		if msg, isStart := c().(promptAlertMsg); isStart {
			startMsg = msg
		}
	}
	if startMsg == nil {
		t.Fatal("the start command was not part of the startup batch")
	}

	// its reported error reaches the prompt like any command error, as an
	// alert activated by the returned command
	updated, alertCmd := model.Update(startMsg)
	model = updated.(*promptModel)
	if alertCmd == nil {
		t.Fatal("the start error does not return an alert command")
	}
	updated, _ = model.Update(alertCmd())
	model = updated.(*promptModel)
	if !strings.Contains(model.View().Content, "topology start failed") {
		t.Fatalf("the start error is not displayed: %q", model.View().Content)
	}
}

func TestPromptClearTranscriptResetsScroll(t *testing.T) {
	var model *promptModel
	model = newPromptModel("[project]> ", nil, func(string) (bool, error) {
		// the submit callback forwards the command queued by clear
		model.pendingCmd = func() tea.Msg { return promptClearMsg{} }
		return false, nil
	})
	model.appendOutput("first output\nsecond output")
	model.transcriptVP.ScrollUp(3)

	// the submitted clear line is echoed, then the command run by the
	// program sends the clear message back to the model
	model.input.SetValue("clear")
	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(*promptModel)
	if len(model.transcript) == 0 {
		t.Fatal("submitted command was not echoed to the transcript")
	}
	if cmd == nil {
		t.Fatal("no command returned to deliver the transcript clear")
	}

	updated, _ = model.Update(cmd())
	model = updated.(*promptModel)

	if len(model.transcript) != 0 {
		t.Fatalf("transcript not cleared: %v", model.transcript)
	}
	if model.transcriptVP.YOffset() != 0 {
		t.Fatalf("scroll offset after clear = %d, want 0", model.transcriptVP.YOffset())
	}
	if view := model.View().Content; strings.Contains(view, "output") {
		t.Fatalf("cleared transcript is still rendered: %q", view)
	}
}

func TestPromptSubPromptListsFilesAndDisplaysContent(t *testing.T) {
	var queued []string
	var selected string
	model := newPromptModel("[project]> ", nil, func(string) (bool, error) { return false, nil })
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
	model = updated.(*promptModel)

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
	model := newPromptModel("[project]> ", nil, func(string) (bool, error) { return false, nil })
	model.startSubPrompt(&inputPromptRequest{
		prefix: "Select configuration file: ",
		submit: func(string) { submitted = true },
	})

	model.input.SetValue("router")
	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	model = updated.(*promptModel)

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

func TestProjectPromptUsesFullscreenAndErrorAlert(t *testing.T) {
	model := newPromptModel("[demo]> ", nil, func(string) (bool, error) { return false, nil })
	model.projectName = "demo"
	model.projectPath = "/work/demo.gnet"
	model.showHeader = true
	model.fullscreen = true
	model.transcript = []string{"Previous output"}

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	model = updated.(*promptModel)
	updated, alertCmd := model.Update(promptAlertMsg{
		alertType: bubbleup.ErrorKey,
		alertMsg:  "command failed",
	})
	model = updated.(*promptModel)
	if alertCmd == nil {
		t.Fatal("no alert command returned for the command error")
	}
	updated, _ = model.Update(alertCmd())
	model = updated.(*promptModel)

	view := model.View()
	if !view.AltScreen {
		t.Fatal("project prompt does not use the alternate fullscreen buffer")
	}
	lines := strings.Split(view.Content, "\n")
	if len(lines) != 12 {
		t.Fatalf("prompt rendered %d lines, want terminal height 12", len(lines))
	}
	for _, text := range []string{
		"Project: demo",
		"Path: /work/demo.gnet",
		"History",
		"[demo]> ",
	} {
		if !strings.Contains(view.Content, text) {
			t.Fatalf("project prompt does not display %q", text)
		}
	}
	// the title is the first line, centered, with the version and project
	if !strings.Contains(lines[0], "gonetem "+options.VERSION) || !strings.Contains(lines[0], "Project: demo") {
		t.Fatalf("title does not display gonetem, its version and the project: %q", lines[0])
	}
	if strings.Index(lines[0], "gonetem "+options.VERSION) < 5 {
		t.Fatalf("title is not displayed at the top center: %q", lines[0])
	}
	if !strings.Contains(lines[len(lines)-1], "Path: /work/demo.gnet") {
		t.Fatalf("the project path is not displayed at the bottom of the terminal: %q", lines[len(lines)-1])
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
	if promptIndex != promptSuggestionLines+1 {
		t.Fatalf("prompt is not right below the reserved suggestion zone: line %d", promptIndex)
	}
	// the error is displayed as an alert overlaid at the bottom center,
	// without displacing the prompt or the footer
	if !model.alert.HasActiveAlert() || !strings.Contains(view.Content, "command failed") {
		t.Fatalf("the error is not displayed as an alert: %q", view.Content)
	}
}

func TestPromptKeepsSuggestionsAboveFixedPrompt(t *testing.T) {
	completions := func(string) []Completion {
		matches := make([]Completion, 0, 10)
		for i := 1; i <= 10; i++ {
			matches = append(matches, Completion{Text: fmt.Sprintf("start node%d", i)})
		}
		return matches
	}
	model := newPromptModel("[demo]> ", completions, func(string) (bool, error) { return false, nil })
	model.showHeader = true
	model.fullscreen = true
	model.projectName = "demo"
	model.projectPath = "/work/demo.gnet"
	model.transcript = []string{"Previous output"}

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	model = updated.(*promptModel)

	promptLine := func(lines []string) int {
		for i, line := range lines {
			if strings.Contains(line, "[demo]> ") {
				return i
			}
		}
		return -1
	}

	before := promptLine(strings.Split(model.View().Content, "\n"))
	if before != promptSuggestionLines+1 {
		t.Fatalf("prompt is not right below the reserved suggestion zone: line %d", before)
	}

	// an in-progress command fills the reserved zone with suggestions,
	// above the prompt, without moving the prompt line
	model.input.SetValue("start ")
	model.updateCompletions()
	lines := strings.Split(model.View().Content, "\n")
	after := promptLine(lines)
	if after != before {
		t.Fatalf("the prompt moved when suggestions appeared: %d -> %d", before, after)
	}
	zone := strings.Join(lines[1:after], "\n")
	if !strings.Contains(zone, "start node1") {
		t.Fatalf("suggestions are not displayed above the prompt: %q", zone)
	}
	// the suggestion zone is fixed: suggestions beyond its space are dropped
	if strings.Contains(zone, "start node10") {
		t.Fatalf("suggestions overflow the reserved zone: %q", zone)
	}

	// the command history is displayed below the prompt, behind its banner
	below := strings.Join(lines[after+1:], "\n")
	bannerAt, outputAt := strings.Index(below, "History"), strings.Index(below, "Previous output")
	if bannerAt == -1 || outputAt == -1 || bannerAt > outputAt {
		t.Fatalf("the history is not displayed below its banner: %q", below)
	}
	if !strings.Contains(lines[len(lines)-1], "Path: /work/demo.gnet") {
		t.Fatalf("the project path moved off the bottom line: %q", lines[len(lines)-1])
	}

	// the progress bars of a running command take the suggestion zone: the
	// suggestions are not displayed at the same time, and the prompt line
	// still keeps its position
	waiting := "Running the topology, please wait until it completes..."
	updated, _ = model.Update(promptBusyMsg{busy: true, message: waiting})
	model = updated.(*promptModel)
	updated, _ = model.Update(promptProgressMsg(
		"Node - Start: [====>        ] 1/2\nNode - Configuration: [==>          ] 1/4"))
	model = updated.(*promptModel)
	lines = strings.Split(model.View().Content, "\n")
	after = promptLine(lines)
	if after != before {
		t.Fatalf("the prompt moved when the progress bars appeared: %d -> %d", before, after)
	}
	zone = strings.Join(lines[1:after], "\n")
	if !strings.Contains(zone, "Node - Start:") || !strings.Contains(zone, "Node - Configuration:") {
		t.Fatalf("the progress bars are not displayed in the suggestion zone: %q", zone)
	}
	if strings.Contains(zone, "start node") {
		t.Fatalf("suggestions are displayed along with the progress bars: %q", zone)
	}
	if waitAt, barAt := strings.Index(zone, waiting), strings.Index(zone, "Node - Start:"); waitAt == -1 || waitAt > barAt {
		t.Fatalf("the waiting message is not above the progress bars: %q", zone)
	}
}

// TestPromptSoftWrapsLongTranscriptLines covers the viewport soft wrapping:
// a transcript line wider than the terminal is displayed on several display
// lines without overflowing the fixed-height layout.
func TestPromptSoftWrapsLongTranscriptLines(t *testing.T) {
	model := newPromptModel("[project]> ", nil, func(string) (bool, error) { return false, nil })
	model.fullscreen = true
	model.transcript = []string{strings.Repeat("x", 200), "after"}

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	model = updated.(*promptModel)

	view := model.View()
	if got := len(strings.Split(view.Content, "\n")); got != 20 {
		t.Fatalf("the long line overflowed the fixed layout: %d lines, want terminal height 20", got)
	}
	if !strings.Contains(view.Content, strings.Repeat("x", 80)) {
		t.Fatal("the long line is not displayed")
	}
	if !strings.Contains(view.Content, "after") {
		t.Fatal("the line after the long one is not displayed")
	}
	if model.transcriptVP.TotalLineCount() <= len(model.transcript) {
		t.Fatalf("the long line was not soft wrapped: total %d for %d transcript lines",
			model.transcriptVP.TotalLineCount(), len(model.transcript))
	}
}

func TestPromptDisplaysCommandErrorAsAlert(t *testing.T) {
	failing := true
	var model *promptModel
	model = newPromptModel("[project]> ", nil, func(string) (bool, error) {
		if failing {
			return false, errors.New("node router1 is not running")
		}
		return false, nil
	})

	model.input.SetValue("start router1")
	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(*promptModel)
	if cmd == nil {
		t.Fatal("no alert command returned for the command error")
	}
	// the batch of a single alert command is collapsed: cmd() sends the
	// alert message directly, as the program would deliver it
	updated, _ = model.Update(cmd())
	model = updated.(*promptModel)

	view := model.View().Content
	if !model.alert.HasActiveAlert() || !strings.Contains(view, "node router1 is not running") {
		t.Fatalf("returned error is not displayed as an alert: %q", view)
	}

	// esc closes a displayed error alert
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	model = updated.(*promptModel)

	if model.alert.HasActiveAlert() {
		t.Fatal("esc does not close the error alert")
	}
}

// TestAlertStaysCenteredOverShortBottomLines covers the bottom-center alert
// over a view whose last lines are the short filler lines and the footer:
// bubbleup centers each alert row against the content line under it and,
// when that line is shorter than the centering offset, pastes the row at
// column 0. View pads every line to the terminal width so the box stays one
// aligned, centered block.
func TestAlertStaysCenteredOverShortBottomLines(t *testing.T) {
	const termWidth = 80
	model := newPromptModel("[demo]> ", nil, func(string) (bool, error) { return false, nil })
	model.fullscreen = true
	model.showHeader = true
	model.projectPath = "/work/demo.gnet"

	updated, _ := model.Update(tea.WindowSizeMsg{Width: termWidth, Height: 12})
	model = updated.(*promptModel)

	// trigger the alert through its message, as the returned NewAlertCmd
	// does when the program runs it
	cmd := model.alert.NewAlertCmd(bubbleup.InfoKey, "command output was saved")
	updated, _ = model.Update(cmd())
	model = updated.(*promptModel)

	content := ansi.Strip(model.View().Content)
	lines := strings.Split(content, "\n")

	// columnAt returns the display column of the first occurrence of char
	// in line, as border and prefix glyphs are multi-byte runes
	columnAt := func(line, char string) int {
		i := strings.Index(line, char)
		if i < 0 {
			return -1
		}
		return lipgloss.Width(line[:i])
	}

	var starts []int
	boxWidth := 0
	for _, line := range lines {
		if corner := columnAt(line, "╭"); corner >= 0 {
			starts = append(starts, corner)
			boxWidth = columnAt(line, "╮") - corner + 1
		} else if side := columnAt(line, "│"); side >= 0 {
			starts = append(starts, side)
		} else if corner := columnAt(line, "╰"); corner >= 0 {
			starts = append(starts, corner)
		}
	}
	// a one line message renders top border, body and bottom border rows
	if len(starts) != 3 {
		t.Fatalf("the alert rendered %d bordered rows, want 3: %q", len(starts), content)
	}
	for _, start := range starts {
		if start != starts[0] {
			t.Fatalf("the alert rows are not aligned: %v in %q", starts, lines)
		}
	}
	// centered: left and right margins are as equal as the box width allows
	if left, right := starts[0], termWidth-starts[0]-boxWidth; left-right > 1 {
		t.Fatalf("the alert is not centered: left margin %d, right margin %d, box width %d",
			left, right, boxWidth)
	}
}

func TestPromptCommandResultOpensModalClosedWithEscape(t *testing.T) {
	var queued []string
	var model *promptModel
	model = newPromptModel("[project]> ", nil, func(string) (bool, error) {
		// the submit callback drains the output queued by the command, as
		// NewPrompt does
		model.drainOutput()
		return false, nil
	})
	model.fullscreen = true
	model.showHeader = true
	model.takeOutput = func() []string {
		output := queued
		queued = nil
		return output
	}

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	model = updated.(*promptModel)

	queued = []string{"Project demo\n- State: Running\n"}
	model.input.SetValue("status")
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(*promptModel)

	// the result is displayed as a bordered modal over the UI
	view := ansi.Strip(model.View().Content)
	if !strings.Contains(view, "State: Running") {
		t.Fatalf("the command result is not displayed: %q", view)
	}
	if !strings.Contains(view, "╭") || !strings.Contains(view, "esc close") {
		t.Fatalf("the command result is not displayed in a modal box: %q", view)
	}
	// it is not appended to the history transcript, only the command echo is
	if len(model.transcript) != 1 || model.transcript[0] != "[project]> status" {
		t.Fatalf("the history transcript should only hold the command echo: %v", model.transcript)
	}

	// while the modal is open, the prompt input is frozen behind it
	updated, _ = model.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	model = updated.(*promptModel)
	if model.input.Value() != "" {
		t.Fatalf("typing is not frozen while the modal is open: %q", model.input.Value())
	}

	// escape closes the modal and frees the prompt again
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	model = updated.(*promptModel)
	if model.modal != nil {
		t.Fatal("escape does not close the result modal")
	}
	if strings.Contains(ansi.Strip(model.View().Content), "State: Running") {
		t.Fatalf("the closed modal is still displayed: %q", model.View().Content)
	}
	updated, _ = model.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	model = updated.(*promptModel)
	if model.input.Value() != "s" {
		t.Fatalf("the prompt is not usable after closing the modal: %q", model.input.Value())
	}
}
