package console

import (
	"context"
	"fmt"
	"strings"

	"github.com/mroy31/gonetem/internal/proto"
)

func StartRemoteConsole(server, node string, shell bool) error {
	terminalFd, err := TermGetFd()
	if err != nil {
		return err
	}

	args := strings.Split(node, ".")
	if len(args) != 2 {
		return fmt.Errorf("%s is not a valid identifier, prj.node expected", node)
	}

	client, err := NewClient(server)
	if err != nil {
		return err
	}

	stream, err := client.Client.NodeConsole(context.Background())
	if err != nil {
		return err
	}
	defer stream.CloseSend()

	if err := stream.Send(&proto.ExecCltMsg{
		Code:  proto.ExecCltMsg_CONSOLE,
		PrjId: args[0],
		Node:  args[1],
		Shell: shell,
	}); err != nil {
		return err
	}

	return monitorExec(stream, terminalFd)
}
