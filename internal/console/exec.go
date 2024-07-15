package console

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/moby/term"
	"github.com/mroy31/gonetem/internal/proto"
)

func termGetTtySize(terminalFd uintptr) (int, int) {
	ws, err := term.GetWinsize(terminalFd)
	if err != nil && ws == nil {
		return 0, 0
	}
	return int(ws.Height), int(ws.Width)
}

func termResizeTty(stream proto.Netem_NodeExecClient, terminalFd uintptr) error {
	height, width := termGetTtySize(terminalFd)
	if height == 0 && width == 0 {
		return nil
	}
	return stream.Send(&proto.ExecCltMsg{
		Code:      proto.ExecCltMsg_RESIZE,
		TtyWidth:  int32(width),
		TtyHeight: int32(height),
	})
}

func termMonitorTty(stream proto.Netem_NodeExecClient, terminalFd uintptr) {
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGWINCH)
	signal.Notify(sigchan, syscall.SIGHUP)
	go func() {
		for sig := range sigchan {
			switch sig {
			case syscall.SIGWINCH:
				termResizeTty(stream, terminalFd)
			case syscall.SIGHUP:
				// terminal has been closed, send exit to
				stream.Send(&proto.ExecCltMsg{
					Code: proto.ExecCltMsg_CLOSE,
				})
			}
		}
	}()
}

func nodeExec(
	client proto.NetemClient,
	prjId string,
	node string,
	cmd []string,
) error {
	var (
		terminalFd uintptr
		out        io.Writer = os.Stdout
		oldState   *term.State
		err        error
	)

	if file, ok := out.(*os.File); ok {
		terminalFd = file.Fd()
	} else {
		return errors.New("not a terminal")
	}

	// Set up the pseudo terminal
	oldState, err = term.SetRawTerminal(terminalFd)
	if err != nil {
		return err
	}

	// Clean up after the command has exited
	defer term.RestoreTerminal(terminalFd, oldState)

	stream, err := client.NodeExec(context.Background())
	if err != nil {
		return err
	}
	defer stream.CloseSend()

	tHeight, tWidth := termGetTtySize(terminalFd)
	if err := stream.Send(&proto.ExecCltMsg{
		Code:      proto.ExecCltMsg_CMD,
		PrjId:     prjId,
		Node:      node,
		Cmd:       cmd,
		Tty:       true,
		TtyHeight: int32(tHeight),
		TtyWidth:  int32(tWidth),
	}); err != nil {
		return err
	}
	stdIn, stdOut, stderr := term.StdStreams()
	waitc := make(chan error)

	// read stdin
	go func() {
		data := make([]byte, 32)
		for {
			n, err := stdIn.Read(data)
			if err != nil {
				waitc <- err
				return
			}

			if err := stream.Send(&proto.ExecCltMsg{
				Code: proto.ExecCltMsg_DATA,
				Data: data[:n],
			}); err != nil {
				stdOut.Write(data[:n])
				if err != io.EOF {
					waitc <- err
				} else {
					waitc <- nil
				}
				return
			}
		}
	}()

	// receive sdtout
	go func() {
		for {
			in, err := stream.Recv()

			if err == io.EOF { // read done.
				waitc <- nil
				return
			}

			if err != nil {
				waitc <- err
				return
			}

			switch in.GetCode() {
			case proto.ExecSrvMsg_CLOSE:
				waitc <- nil
				return
			case proto.ExecSrvMsg_ERROR:
				// the serveur return an error
				waitc <- fmt.Errorf("%s", string(in.GetData()))
				return
			case proto.ExecSrvMsg_STDOUT:
				if _, err := stdOut.Write(in.GetData()); err != nil {
					fmt.Printf("Error os.Stdout.Write: %v\n", err)
					waitc <- err
					return
				}
			case proto.ExecSrvMsg_STDERR:
				if _, err := stderr.Write(in.GetData()); err != nil {
					fmt.Printf("Error os.Stdout.Write: %v\n", err)
					waitc <- err
					return
				}
			}

		}
	}()

	termMonitorTty(stream, terminalFd)

	return <-waitc
}
