package server

import (
	"fmt"
	"io"
	"regexp"

	"github.com/moby/term"
	"github.com/mroy31/gonetem/internal/docker"
	"github.com/mroy31/gonetem/internal/link"
	"github.com/mroy31/gonetem/internal/ovs"
	"github.com/vishvananda/netns"
)

type INetemNode interface {
	GetName() string
	GetType() string
	IsRunning() bool
	Start() error
	Stop() error
	GetNetns() (netns.NsHandle, error)
	GetInterfaceName(ifIndex int) string
	AddInterface(ifName string, ifIndex int, ns netns.NsHandle) error
	GetInterfaces() map[string]link.IfState
	LoadConfig(confPath string) error
	CanRunConsole() error
	Console(shell bool, in io.ReadCloser, out io.Writer, resizeCh chan term.Winsize) error
	Capture(ifIndex int, out io.Writer) error
	Save(dstPath string) error
	SetInterfaceState(ifIndex int, state link.IfState) error
	Close() error
}

type NodeNotFoundError struct {
	prjId string
	name  string
}

func (e *NodeNotFoundError) Error() string {
	return fmt.Sprintf("Node %s not found in project %s", e.name, e.prjId)
}

func CreateNode(prjID string, name string, config NodeConfig) (INetemNode, error) {
	// first test if it is a docker node
	re := regexp.MustCompile(`^docker.(\w+)$`)
	groups := re.FindStringSubmatch(config.Type)
	if len(groups) == 2 {
		// Create docker node
		return docker.NewDockerNode(prjID, docker.DockerNodeOptions{
			Name: name,
			Type: groups[1],
			Ipv6: config.IPv6,
			Mpls: config.Mpls,
			Vrfs: config.Vrfs,
		})
	}

	// then test if it is a switch
	if config.Type == "ovs" {
		return ovs.NewOvsNode(prjID, name)
	}

	return nil, fmt.Errorf("Unknown node type '%s'", config.Type)
}
