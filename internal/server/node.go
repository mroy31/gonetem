package server

import (
	"fmt"
	"io"
	"regexp"
	"sync"

	"github.com/moby/term"
	"github.com/mroy31/gonetem/internal/docker"
	"github.com/mroy31/gonetem/internal/link"
	"github.com/mroy31/gonetem/internal/ovs"
	"github.com/vishvananda/netns"
)

type INetemNode interface {
	GetName() string
	GetShortName() string
	GetType() string
	IsRunning() bool
	Start() error
	Stop() error
	GetNetns() (netns.NsHandle, error)
	GetInterfaceName(ifIndex int) string
	AddInterface(ifName string, ifIndex int, ns netns.NsHandle) error
	LoadConfig(confPath string) ([]string, error)
	CanRunConsole() error
	Console(shell bool, in io.ReadCloser, out io.Writer, resizeCh chan term.Winsize) error
	Capture(ifIndex int, out io.Writer) error
	CopyFrom(srcPath, destPath string) error
	CopyTo(srcPath, destPath string) error
	Save(dstPath string) error
	GetInterfacesState() map[string]link.IfState
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

type NodeIdentifierGenerator struct {
	lock    *sync.Mutex
	usedIds []string
}

func (nIdGen *NodeIdentifierGenerator) isIdExist(id string) bool {
	for _, nId := range nIdGen.usedIds {
		if nId == id {
			return true
		}
	}
	return false
}

func (nIdGen *NodeIdentifierGenerator) GetId(name string) (string, error) {
	genId := ""
	if len(name) <= 5 {
		genId = name
	} else {
		genId = name[:2] + name[len(name)-2:]
	}

	nIdGen.lock.Lock()
	defer nIdGen.lock.Unlock()

	for idx := range [10]int{} {
		genId := fmt.Sprintf("%d%s", idx, genId)
		if !nIdGen.isIdExist(genId) {
			nIdGen.usedIds = append(nIdGen.usedIds, genId)
			return genId, nil
		}
	}

	return "", fmt.Errorf("Unable to generate a short id for node %s: all attempts fail", name)
}

func (nIdGen *NodeIdentifierGenerator) Close() {
	nIdGen.lock.Lock()
	defer nIdGen.lock.Unlock()

	nIdGen.usedIds = make([]string, 0)
}

func CreateNode(prjID string, name string, shortName string, config NodeConfig) (INetemNode, error) {
	// first test if it is a docker node
	re := regexp.MustCompile(`^docker.(\w+)$`)
	groups := re.FindStringSubmatch(config.Type)
	if len(groups) == 2 {
		// Create docker node
		options := docker.DockerNodeOptions{
			Name:      name,
			ShortName: shortName,
			ImgName:   config.Image,
			Type:      groups[1],
			Ipv6:      config.IPv6,
			Mpls:      config.Mpls,
			Vrfs:      config.Vrfs,
		}
		for _, group := range config.Vrrps {
			options.Vrrps = append(options.Vrrps, docker.VrrpOptions{
				Interface: group.Interface,
				Group:     group.Group,
				Address:   group.Address,
			})
		}

		return docker.NewDockerNode(prjID, options)
	}

	// then test if it is a switch
	if config.Type == "ovs" {
		return ovs.NewOvsNode(prjID, name, shortName)
	}

	return nil, fmt.Errorf("Unknown node type '%s'", config.Type)
}
