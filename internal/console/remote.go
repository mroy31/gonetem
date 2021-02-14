package console

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/moby/term"
	"github.com/mroy31/gonetem/internal/proto"
)

func StartRemoteConsole(server, node string) error {
	var (
		terminalFd uintptr
		oldState   *term.State
		out        io.Writer = os.Stdout
	)

	if file, ok := out.(*os.File); ok {
		terminalFd = file.Fd()
	} else {
		return errors.New("Not a terminal!")
	}

	args := strings.Split(node, ".")
	if len(args) != 2 {
		return fmt.Errorf("%s is not a valid identifier, prj.node expected", node)
	}

	client, err := NewClient(server)
	if err != nil {
		return err
	}

	stream, err := client.Client.Console(context.Background())
	if err != nil {
		return err
	}
	defer stream.CloseSend()

	if err := stream.Send(&proto.ConsoleCltMsg{
		Code:  proto.ConsoleCltMsg_INIT,
		PrjId: args[0],
		Node:  args[1],
	}); err != nil {
		return err
	}

	// Set up the pseudo terminal
	oldState, err = term.SetRawTerminal(terminalFd)
	if err != nil {
		return err
	}

	// Clean up after the command has exited
	defer term.RestoreTerminal(terminalFd, oldState)

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

			if err := stream.Send(&proto.ConsoleCltMsg{
				Code: proto.ConsoleCltMsg_DATA,
				Data: data[:n],
			}); err != nil {
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

			if err == io.EOF {
				// read done.
				waitc <- nil
				return
			}

			if err != nil {
				waitc <- err
				return
			}

			switch in.GetCode() {
			case proto.ConsoleSrvMsg_CLOSE:
				waitc <- nil
				return
			case proto.ConsoleSrvMsg_ERROR:
				// the serveur return an error
				waitc <- fmt.Errorf("%s", string(in.GetData()))
				return
			case proto.ConsoleSrvMsg_STDOUT:
				if _, err := stdOut.Write(in.GetData()); err != nil {
					fmt.Printf("Error os.Stdout.Write: %v\n", err)
					waitc <- err
					return
				}
			case proto.ConsoleSrvMsg_STDERR:
				if _, err := stderr.Write(in.GetData()); err != nil {
					fmt.Printf("Error os.Stdout.Write: %v\n", err)
					waitc <- err
					return
				}
			}

		}
	}()

	monitorTty(stream, terminalFd)

	return <-waitc
}

func monitorTty(stream proto.Netem_ConsoleClient, terminalFd uintptr) {
	resizeTty(stream, terminalFd)

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGWINCH)
	go func() {
		for _ = range sigchan {
			resizeTty(stream, terminalFd)
		}
	}()
}

func resizeTty(stream proto.Netem_ConsoleClient, terminalFd uintptr) error {
	height, width := getTtySize(terminalFd)
	if height == 0 && width == 0 {
		return nil
	}
	return stream.Send(&proto.ConsoleCltMsg{
		Code:      proto.ConsoleCltMsg_RESIZE,
		TtyWidth:  int32(width),
		TtyHeight: int32(height),
	})
}

func getTtySize(terminalFd uintptr) (int, int) {
	ws, err := term.GetWinsize(terminalFd)
	if err != nil && ws == nil {
		return 0, 0
	}
	return int(ws.Height), int(ws.Width)
}
