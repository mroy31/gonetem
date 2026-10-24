package console

import (
	"fmt"
	"strings"

	"github.com/mroy31/gonetem/internal/proto"
)

// progressBarWidth is the size of the bar rendered by progressText.
const progressBarWidth = 50

// progressText renders a simple text progress bar, as displayed inside the
// TUI prompt: bubbletea owns the terminal, so the bars must go through it.
func progressText(label string, current, total int) string {
	if total <= 0 {
		return label + " ..."
	}

	width := progressBarWidth
	filled := min(width, current*width/total)
	return fmt.Sprintf("%s [%s%s] %d/%d", label,
		strings.Repeat("█", filled), strings.Repeat("░", width-filled), current, total)
}

// Labels of the topology run phases, displayed as they progress by the TUI
// text bars during the run, reload and exit commands.
const (
	runLabelNodeClose    = "Node - Close"
	runLabelBridgeClose  = "Bridge - Close"
	runLabelNodeStart    = "Node - Start"
	runLabelNodeConfig   = "Node - Configuration"
	runLabelLinkCreate   = "Link - Create"
	runLabelBridgeCreate = "Bridge - Create"
)

// nodeMessagesOutput renders the per-node warnings reported while running a
// topology, appended to the prompt history by the TUI run path.
func nodeMessagesOutput(msg *proto.TopologyRunMsg) string {
	var output strings.Builder
	for _, nMessages := range msg.GetNodeMessages() {
		if len(nMessages.GetMessages()) == 0 {
			continue
		}
		fmt.Fprintln(&output, nMessages.GetName()+":")
		for _, msg := range nMessages.GetMessages() {
			if msg != "" {
				fmt.Fprintln(&output, "  - "+msg)
			}
		}
	}
	return output.String()
}

// progressCounters tracks the phase counters of a server stream and renders
// them as text progress bars for the TUI prompt. A phase is only displayed
// between its announcement and its end.
type progressCounters struct {
	counters []*runProgressCounter
}

// runProgress tracks the phases of the topology run streams, run and reload.
type runProgress struct {
	progressCounters
	// closing is true while a reload stream runs its initial close phase:
	// the NODE_COUNT and BRIDGE_COUNT announcements of this phase come
	// before any increment, and are followed by the run phase announcing
	// its own counts.
	closing bool
}

type runProgressCounter struct {
	label   string
	current int
	total   int
}

func newRunProgress() *runProgress {
	return &runProgress{progressCounters: progressCounters{counters: []*runProgressCounter{
		{label: runLabelNodeStart},
		{label: runLabelNodeConfig},
		{label: runLabelLinkCreate},
		{label: runLabelBridgeCreate},
	}}}
}

// newReloadProgress tracks the additional close phase of the reload
// stream, which stops the nodes and bridges before the run phase restarts
// them. The close bars are displayed above the run ones.
func newReloadProgress() *runProgress {
	return &runProgress{
		progressCounters: progressCounters{counters: append([]*runProgressCounter{
			{label: runLabelNodeClose},
			{label: runLabelBridgeClose},
		}, newRunProgress().counters...)},
		closing: true,
	}
}

// closeProgress tracks the phases of the project close stream, the text
// bars displayed by the exit command while it closes the project inside
// the TUI.
type closeProgress struct {
	progressCounters
}

func newCloseProgress() *closeProgress {
	return &closeProgress{progressCounters: progressCounters{counters: []*runProgressCounter{
		{label: runLabelNodeClose},
		{label: runLabelBridgeClose},
	}}}
}

// handle applies a close stream message to the counters and returns the
// progress text to display.
func (c *closeProgress) handle(msg *proto.ProjectCloseMsg) string {
	switch msg.GetCode() {
	case proto.ProjectCloseMsg_NODE_COUNT:
		c.setTotal(runLabelNodeClose, int(msg.GetTotal()))
	case proto.ProjectCloseMsg_BRIDGE_COUNT:
		c.setTotal(runLabelBridgeClose, int(msg.GetTotal()))
	case proto.ProjectCloseMsg_NODE_CLOSE:
		c.increment(runLabelNodeClose)
	case proto.ProjectCloseMsg_BRIDGE_CLOSE:
		c.increment(runLabelBridgeClose)
	}
	return c.text()
}

// handle applies a stream message to the counters and returns the progress
// text to display: the running phases, one per line, empty when none is.
func (r *runProgress) handle(msg *proto.TopologyRunMsg) string {
	switch msg.GetCode() {
	case proto.TopologyRunMsg_NODE_COUNT:
		if r.closing {
			r.setTotal(runLabelNodeClose, int(msg.GetTotal()))
		} else {
			r.setTotal(runLabelNodeStart, int(msg.GetTotal()))
			r.setTotal(runLabelNodeConfig, int(msg.GetTotal()))
		}
	case proto.TopologyRunMsg_BRIDGE_COUNT:
		if r.closing {
			r.setTotal(runLabelBridgeClose, int(msg.GetTotal()))
		} else {
			r.setTotal(runLabelBridgeCreate, int(msg.GetTotal()))
		}
	case proto.TopologyRunMsg_LINK_COUNT:
		r.setTotal(runLabelLinkCreate, int(msg.GetTotal()))
	case proto.TopologyRunMsg_NODE_STOP:
		r.increment(runLabelNodeClose)
		r.closing = false
	case proto.TopologyRunMsg_NODE_RM:
		r.increment(runLabelBridgeClose)
		r.closing = false
	case proto.TopologyRunMsg_NODE_START:
		r.increment(runLabelNodeStart)
	case proto.TopologyRunMsg_NODE_LOADCONFIG:
		r.increment(runLabelNodeConfig)
	case proto.TopologyRunMsg_LINK_SETUP:
		r.increment(runLabelLinkCreate)
	case proto.TopologyRunMsg_BRIDGE_START:
		r.increment(runLabelBridgeCreate)
	case proto.TopologyRunMsg_NODE_MESSAGES:
		r.forceComplete()
	}

	return r.text()
}

func (c *progressCounters) counter(label string) *runProgressCounter {
	for _, counter := range c.counters {
		if counter.label == label {
			return counter
		}
	}
	return nil
}

func (c *progressCounters) setTotal(label string, total int) {
	if counter := c.counter(label); counter != nil && total > 0 {
		counter.total = total
	}
}

func (c *progressCounters) increment(label string) {
	// increments of a phase not announced yet by its *_COUNT message are
	// ignored, as the phase has not started yet
	if counter := c.counter(label); counter != nil && counter.total > 0 {
		counter.current++
	}
}

// forceComplete ends all phases, so the completed run clears the bars when
// the node warnings take over the display.
func (c *progressCounters) forceComplete() {
	for _, counter := range c.counters {
		counter.current = counter.total
	}
}

// text renders the running phases, one per line, empty when none is.
func (c *progressCounters) text() string {
	lines := make([]string, 0, len(c.counters))
	for _, counter := range c.counters {
		if counter.total > 0 && counter.current < counter.total {
			lines = append(lines, progressText(counter.label+":", counter.current, counter.total))
		}
	}
	return strings.Join(lines, "\n")
}
