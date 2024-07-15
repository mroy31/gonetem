package console

import (
	"context"
	"fmt"
	"strings"

	"github.com/mroy31/gonetem/internal/proto"
)

func StartRemoteConsole(server, node string, shell bool) error {
	args := strings.Split(node, ".")
	if len(args) != 2 {
		return fmt.Errorf("%s is not a valid identifier, prj.node expected", node)
	}

	client, err := NewClient(server)
	if err != nil {
		return err
	}
	defer client.Conn.Close()

	cmd, err := client.Client.NodeGetConsoleCmd(
		context.Background(),
		&proto.ConsoleCmdRequest{
			PrjId: args[0],
			Node:  args[1],
			Shell: shell,
		},
	)
	if err != nil {
		return err
	}

	return nodeExec(client.Client, args[0], args[1], cmd.GetCmd())
}
