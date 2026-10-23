package console

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/mroy31/gonetem/internal/proto"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

// progressBarWidth is the size of the bar rendered by progressText.
const progressBarWidth = 50

// progressText renders a simple text progress bar to display inside the
// TUI prompt, where mpb bars cannot be used as they write directly to the
// terminal owned by bubbletea.
func progressText(label string, current, total int) string {
	if total <= 0 {
		return label + " ..."
	}

	width := progressBarWidth
	filled := min(width, current*width/total)
	return fmt.Sprintf("%s [%s%s] %d/%d", label,
		strings.Repeat("█", filled), strings.Repeat("░", width-filled), current, total)
}

// Labels of the topology run phases, displayed as they progress by both
// the mpb bars and the TUI text bars.
const (
	runLabelNodeClose    = "Node - Close"
	runLabelBridgeClose  = "Bridge - Close"
	runLabelNodeStart    = "Node - Start"
	runLabelNodeConfig   = "Node - Configuration"
	runLabelLinkCreate   = "Link - Create"
	runLabelBridgeCreate = "Bridge - Create"
)

type ProgressBarT struct {
	Total int
	Bar   *mpb.Bar
}

func ProgressForceComplete(bars []ProgressBarT) {
	for _, bar := range bars {
		if bar.Bar != nil {
			bar.Bar.SetCurrent(int64(bar.Total))
			bar.Bar.Wait()
		}
	}
}

func ProgressAbort(bars []ProgressBarT, drop bool) {
	for _, bar := range bars {
		if bar.Bar != nil {
			bar.Bar.Abort(drop)
			bar.Bar.Wait()
		}
	}
}

func ProgressRunHandleMsg(mpBar *mpb.Progress, bars []ProgressBarT, msg *proto.TopologyRunMsg) {
	switch msg.Code {
	case proto.TopologyRunMsg_NODE_COUNT:
		if msg.Total > 0 {
			bars[0] = ProgressBarT{
				Total: int(msg.Total),
				Bar: mpBar.AddBar(int64(msg.Total),
					mpb.BarRemoveOnComplete(),
					mpb.PrependDecorators(decor.Counters(0, runLabelNodeStart+": %d/%d")),
				),
			}
			bars[1] = ProgressBarT{
				Total: int(msg.Total),
				Bar: mpBar.AddBar(int64(msg.Total),
					mpb.BarQueueAfter(bars[0].Bar),
					mpb.BarRemoveOnComplete(),
					mpb.PrependDecorators(decor.Counters(0, runLabelNodeConfig+": %d/%d")),
				),
			}
		}

	case proto.TopologyRunMsg_LINK_COUNT:
		if msg.Total > 0 {
			bars[2] = ProgressBarT{
				Total: int(msg.Total),
				Bar: mpBar.AddBar(int64(msg.Total),
					mpb.BarRemoveOnComplete(),
					mpb.PrependDecorators(decor.Counters(0, runLabelLinkCreate+": %d/%d")),
				),
			}
		}

	case proto.TopologyRunMsg_BRIDGE_COUNT:
		if msg.Total > 0 {
			bars[3] = ProgressBarT{
				Total: int(msg.Total),
				Bar: mpBar.AddBar(int64(msg.Total),
					mpb.BarRemoveOnComplete(),
					mpb.PrependDecorators(decor.Counters(0, runLabelBridgeCreate+": %d/%d")),
				),
			}
		}

	case proto.TopologyRunMsg_LINK_SETUP:
		bars[2].Bar.Increment()

	case proto.TopologyRunMsg_BRIDGE_START:
		bars[3].Bar.Increment()

	case proto.TopologyRunMsg_NODE_START:
		bars[0].Bar.Increment()

	case proto.TopologyRunMsg_NODE_LOADCONFIG:
		bars[1].Bar.Increment()

	case proto.TopologyRunMsg_NODE_MESSAGES:
		ProgressForceComplete(bars)
		mpBar.Wait()

		fmt.Print(nodeMessagesOutput(msg))
	}
}

// nodeMessagesOutput renders the per-node warnings reported while running a
// topology, shared by the mpb and TUI display paths.
func nodeMessagesOutput(msg *proto.TopologyRunMsg) string {
	var output strings.Builder
	for _, nMessages := range msg.GetNodeMessages() {
		if len(nMessages.GetMessages()) == 0 {
			continue
		}
		fmt.Fprintln(&output, color.YellowString(nMessages.GetName()+":"))
		for _, msg := range nMessages.GetMessages() {
			if msg != "" {
				fmt.Fprintln(&output, color.YellowString("  - "+msg))
			}
		}
	}
	return output.String()
}

// runProgress tracks the phase counters of the topology run stream and
// renders them as text progress bars for the TUI prompt, where mpb bars
// cannot be used. Like the mpb bars removed on completion, a phase is only
// displayed between its announcement and its end.
type runProgress struct {
	counters []*runProgressCounter
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
	return &runProgress{counters: []*runProgressCounter{
		{label: runLabelNodeStart},
		{label: runLabelNodeConfig},
		{label: runLabelLinkCreate},
		{label: runLabelBridgeCreate},
	}}
}

// newReloadProgress tracks the additional close phase of the reload
// stream, which stops the nodes and bridges before the run phase restarts
// them. The close bars are displayed above the run ones.
func newReloadProgress() *runProgress {
	return &runProgress{
		counters: append([]*runProgressCounter{
			{label: runLabelNodeClose},
			{label: runLabelBridgeClose},
		}, newRunProgress().counters...),
		closing: true,
	}
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

	lines := make([]string, 0, len(r.counters))
	for _, counter := range r.counters {
		if counter.total > 0 && counter.current < counter.total {
			lines = append(lines, progressText(counter.label+":", counter.current, counter.total))
		}
	}
	return strings.Join(lines, "\n")
}

func (r *runProgress) counter(label string) *runProgressCounter {
	for _, counter := range r.counters {
		if counter.label == label {
			return counter
		}
	}
	return nil
}

func (r *runProgress) setTotal(label string, total int) {
	if counter := r.counter(label); counter != nil && total > 0 {
		counter.total = total
	}
}

func (r *runProgress) increment(label string) {
	// increments of a phase not announced yet by its *_COUNT message are
	// ignored, as the corresponding mpb bar would not exist either
	if counter := r.counter(label); counter != nil && counter.total > 0 {
		counter.current++
	}
}

// forceComplete ends all phases, as ProgressForceComplete does for the bars.
func (r *runProgress) forceComplete() {
	for _, counter := range r.counters {
		counter.current = counter.total
	}
}

func ProgressCloseHandleMsg(mpBar *mpb.Progress, bars []ProgressBarT, msg *proto.ProjectCloseMsg) {
	switch msg.Code {
	case proto.ProjectCloseMsg_NODE_COUNT:
		if msg.Total > 0 {
			bars[0] = ProgressBarT{
				Total: int(msg.Total),
				Bar: mpBar.AddBar(int64(msg.Total),
					mpb.BarRemoveOnComplete(),
					mpb.PrependDecorators(decor.Counters(0, "Close nodes: %d/%d")),
				),
			}
		}

	case proto.ProjectCloseMsg_BRIDGE_COUNT:
		if msg.Total > 0 {
			bars[1] = ProgressBarT{
				Total: int(msg.Total),
				Bar: mpBar.AddBar(int64(msg.Total),
					mpb.BarRemoveOnComplete(),
					mpb.PrependDecorators(decor.Counters(0, "Bridge start: %d/%d")),
				),
			}
		}

	case proto.ProjectCloseMsg_NODE_CLOSE:
		bars[0].Bar.Increment()

	case proto.ProjectCloseMsg_BRIDGE_CLOSE:
		bars[1].Bar.Increment()
	}
}
