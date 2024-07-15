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

var (
	characterList = []string{
		"0", "1", "2", "3", "4", "5", "6", "7", "8", "9",
		"a", "b", "c", "d", "e", "f", "g", "h", "i", "j",
		"k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "x", "y", "z",
	}
)

type INetemNode interface {
	GetName() string
	GetShortName() string
	GetType() string
	GetFullType() string
	IsRunning() bool
	Start() error
	Stop() error
	GetNetns() (netns.NsHandle, error)
	GetInterfaceName(ifIndex int) string
	AddInterface(ifName string, ifIndex int, ns netns.NsHandle) error
	LoadConfig(confPath string, timeout int) ([]string, error)
	ExecCommand(cmd []string, in io.ReadCloser, out io.Writer, tty bool, ttyHeight uint, ttyWidth uint, resizeCh chan term.Winsize) error
	GetConsoleCmd(shell bool) []string
	Capture(ifIndex int, out io.Writer) error
	CopyFrom(srcPath, destPath string) error
	CopyTo(srcPath, destPath string) error
	ReadConfigFiles(confDir string, timeout int) (map[string][]byte, error)
	Save(dstPath string, timeout int) error
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
	nIdGen.lock.Lock()
	defer nIdGen.lock.Unlock()

	genId := ""
	if len(name) <= 5 {
		genId = name
		if !nIdGen.isIdExist(genId) {
			nIdGen.usedIds = append(nIdGen.usedIds, genId)
			return genId, nil
		}
	}

	for _, idx := range [3]int{2, 3, 1} {
		genId = name[:idx] + name[len(name)-(4-idx):]
		for _, char := range characterList {
			genId := fmt.Sprintf("%s%s", char, genId)
			if !nIdGen.isIdExist(genId) {
				nIdGen.usedIds = append(nIdGen.usedIds, genId)
				return genId, nil
			}
		}
	}

	return "", fmt.Errorf("unable to generate a short id for node %s: all attempts fail", name)
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
			Volumes:   config.Volumes,
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

	return nil, fmt.Errorf("unknown node type '%s'", config.Type)
}
