package server

import (
	"fmt"
	"io"
	"regexp"

	"github.com/moby/term"
	"github.com/mroy31/gonetem/internal/docker"
)

type INetemNode interface {
	GetName() string
	GetType() string
	IsRunning() bool
	Start() error
	Stop() error
	LoadConfig(confPath string) error
	Console(in io.ReadCloser, out io.Writer, resizeCh chan term.Winsize) error
	Save(dstPath string) error
	Close() error
}

func CreateNode(prjID string, config NodeConfig) (INetemNode, error) {
	re := regexp.MustCompile(`^docker.(\w+)$`)
	groups := re.FindStringSubmatch(config.Type)
	if len(groups) == 2 {
		// Create docker node
		return docker.CreateDockerNode(prjID, docker.DockerNodeOptions{
			Name: config.Name,
			Type: groups[1],
			Ipv6: config.IPv6,
			Mpls: config.Mpls,
		})
	}

	return nil, fmt.Errorf("Unknown node type '%s'", config.Type)
}
