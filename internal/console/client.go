package console

import (
	"time"

	"github.com/mroy31/gonetem/internal/proto"
	"google.golang.org/grpc"
)

type NetemConsoleClient struct {
	Conn   *grpc.ClientConn
	Client proto.NetemClient
}

func NewClient(server string) (*NetemConsoleClient, error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())
	opts = append(opts, grpc.WithBlock())
	opts = append(opts, grpc.WithTimeout(500*time.Millisecond))

	conn, err := grpc.Dial(server, opts...)
	if err != nil {
		return nil, err
	}

	return &NetemConsoleClient{
		Conn:   conn,
		Client: proto.NewNetemClient(conn),
	}, nil
}
