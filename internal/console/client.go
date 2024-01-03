package console

import (
	"context"
	"fmt"
	"time"

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

	opts = append(opts, grpc.WithBlock())
	if options.ConsoleConfig.Tls.Enabled {
		var err error

		creds, err = options.LoadConsoleTLSCredentials()
		if err != nil {
			return nil, fmt.Errorf("cannot load TLS credentials: %w", err)
		}
	}
	opts = append(opts, grpc.WithTransportCredentials(creds))

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	conn, err := grpc.DialContext(ctx, server, opts...)
	if err != nil {
		return nil, err
	}

	return &NetemConsoleClient{
		Conn:   conn,
		Client: proto.NewNetemClient(conn),
	}, nil
}
