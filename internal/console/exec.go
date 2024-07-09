package console

import (
	"fmt"
	"io"

	"github.com/moby/term"
	"github.com/mroy31/gonetem/internal/proto"
)

type INetemNodeExecClient interface {
	Send(*proto.ExecCltMsg) error
	Recv() (*proto.ExecSrvMsg, error)
}

func monitorExec(stream INetemNodeExecClient, terminalFd uintptr) error {
	var (
		oldState *term.State
		err      error
	)

	if terminalFd != 0 {
		// Set up the pseudo terminal
		oldState, err = term.SetRawTerminal(terminalFd)
		if err != nil {
			return err
		}

		// Clean up after the command has exited
		defer term.RestoreTerminal(terminalFd, oldState)
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

	if terminalFd != 0 {
		TermMonitorTty(stream, terminalFd)
	}

	return <-waitc
}
