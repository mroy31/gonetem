package console

import (
	"errors"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/moby/term"
	"github.com/mroy31/gonetem/internal/proto"
)

func TermGetFd() (uintptr, error) {
	var (
		terminalFd uintptr
		out        io.Writer = os.Stdout
	)

	if file, ok := out.(*os.File); ok {
		terminalFd = file.Fd()
	} else {
		return terminalFd, errors.New("not a terminal")
	}

	return terminalFd, nil
}

func TermMonitorTty(stream INetemNodeExecClient, terminalFd uintptr) {
	TermResizeTty(stream, terminalFd)

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGWINCH)
	signal.Notify(sigchan, syscall.SIGHUP)
	go func() {
		for sig := range sigchan {
			switch sig {
			case syscall.SIGWINCH:
				TermResizeTty(stream, terminalFd)
			case syscall.SIGHUP:
				// terminal has been closed, send exit to
				stream.Send(&proto.ExecCltMsg{
					Code: proto.ExecCltMsg_CLOSE,
				})
			}
		}
	}()
}

func TermResizeTty(stream INetemNodeExecClient, terminalFd uintptr) error {
	height, width := TermGetTtySize(terminalFd)
	if height == 0 && width == 0 {
		return nil
	}
	return stream.Send(&proto.ExecCltMsg{
		Code:      proto.ExecCltMsg_RESIZE,
		TtyWidth:  int32(width),
		TtyHeight: int32(height),
	})
}

func TermGetTtySize(terminalFd uintptr) (int, int) {
	ws, err := term.GetWinsize(terminalFd)
	if err != nil && ws == nil {
		return 0, 0
	}
	return int(ws.Height), int(ws.Width)
}
