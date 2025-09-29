package console

import (
	"fmt"

	"github.com/mroy31/gonetem/internal/options"
	"github.com/mroy31/gonetem/internal/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type NetemConsoleClient struct {
	Conn   *grpc.ClientConn
	Client proto.NetemClient
}

func NewClient(server string) (*NetemConsoleClient, error) {
	var creds = insecure.NewCredentials()
	var opts []grpc.DialOption

	if options.ConsoleConfig.Tls.Enabled {
		var err error

		creds, err = options.LoadConsoleTLSCredentials()
		if err != nil {
			return nil, fmt.Errorf("cannot load TLS credentials: %w", err)
		}
	}
	opts = append(opts, grpc.WithTransportCredentials(creds))

	conn, err := grpc.NewClient(server, opts...)
	if err != nil {
		return nil, err
	}

	return &NetemConsoleClient{
		Conn:   conn,
		Client: proto.NewNetemClient(conn),
	}, nil
}
