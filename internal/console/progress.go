package console

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/mroy31/gonetem/internal/proto"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
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
					mpb.PrependDecorators(decor.Counters(0, "Node - Start: %d/%d")),
				),
			}
			bars[1] = ProgressBarT{
				Total: int(msg.Total),
				Bar: mpBar.AddBar(int64(msg.Total),
					mpb.BarQueueAfter(bars[0].Bar),
					mpb.BarRemoveOnComplete(),
					mpb.PrependDecorators(decor.Counters(0, "Node - Configuration: %d/%d")),
				),
			}
		}

	case proto.TopologyRunMsg_LINK_COUNT:
		if msg.Total > 0 {
			bars[2] = ProgressBarT{
				Total: int(msg.Total),
				Bar: mpBar.AddBar(int64(msg.Total),
					mpb.BarRemoveOnComplete(),
					mpb.PrependDecorators(decor.Counters(0, "Link - Create: %d/%d")),
				),
			}
		}

	case proto.TopologyRunMsg_BRIDGE_COUNT:
		if msg.Total > 0 {
			bars[3] = ProgressBarT{
				Total: int(msg.Total),
				Bar: mpBar.AddBar(int64(msg.Total),
					mpb.BarRemoveOnComplete(),
					mpb.PrependDecorators(decor.Counters(0, "Bridge - Create: %d/%d")),
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

		for _, nMessages := range msg.NodeMessages {
			if len(nMessages.Messages) > 0 {
				fmt.Println(color.YellowString(nMessages.Name + ":"))
				for _, msg := range nMessages.Messages {
					fmt.Println(color.YellowString("  - " + msg))
				}
			}
		}
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
