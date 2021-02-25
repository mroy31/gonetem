package server

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/docker/docker/pkg/system"
	"github.com/mroy31/gonetem/internal/docker"
	"github.com/mroy31/gonetem/internal/link"
	"github.com/mroy31/gonetem/internal/options"
	"github.com/mroy31/gonetem/internal/ovs"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

func shortName(name string) string {
	if len(name) <= 4 {
		return name
	}

	return name[len(name)-4:]
}

func splitCopyArg(arg string) (container, path string) {
	if system.IsAbs(arg) {
		return "", arg
	}

	parts := strings.SplitN(arg, ":", 2)

	if len(parts) == 1 || strings.HasPrefix(parts[0], ".") {
		// Either there's no `:` in the arg
		// OR it's an explicit local relative path like `./file:name.txt`.
		return "", arg
	}

	return parts[0], parts[1]
}

type copyDirection int

const (
	fromContainer copyDirection = 1 << iota
	toContainer
	acrossContainers = fromContainer | toContainer
)

const (
	networkFilename = "network.yml"
	configDir       = "configs"
)

type NodeConfig struct {
	Name       string
	Type       string
	Interfaces map[string]string
	IPv6       bool
	Mpls       bool
}

type LinkConfig struct {
	Peer1 string
	Peer2 string
}

type BridgeConfig struct {
	Name       string
	Host       string
	Interfaces []string
}

type NetemTopology struct {
	Nodes   []NodeConfig
	Links   []LinkConfig
	Bridges []BridgeConfig
}

type NetemLinkPeer struct {
	Node    INetemNode
	IfIndex int
}

type NetemLink struct {
	Peer1 NetemLinkPeer
	Peer2 NetemLinkPeer
}

type NetemBridge struct {
	Name          string
	HostInterface string
	Peers         []NetemLinkPeer
}

type NetemTopologyManager struct {
	prjID string
	path  string

	nodes       []INetemNode
	ovsInstance *ovs.OvsProjectInstance
	links       []*NetemLink
	bridges     []*NetemBridge
	running     bool
	logger      *logrus.Entry
}

func (t *NetemTopologyManager) Check() error {
	filepath := path.Join(t.path, networkFilename)
	_, errors := CheckTopology(filepath)
	if len(errors) > 0 {
		msg := ""
		for _, err := range errors {
			msg += "\n\t" + err.Error()
		}
		return fmt.Errorf("Topology if not valid:%s\n", msg)
	}

	return nil
}

func (t *NetemTopologyManager) Load() error {
	filepath := path.Join(t.path, networkFilename)
	topology, errors := CheckTopology(filepath)
	if len(errors) > 0 {
		msg := ""
		for _, err := range errors {
			msg += "\n\t" + err.Error()
		}
		return fmt.Errorf("Topology if not valid:%s\n", msg)
	}

	var err error
	// Create openvswitch instance for this project
	t.ovsInstance, err = ovs.NewOvsInstance(t.prjID)
	if err != nil {
		return err
	}

	// Create nodes
	t.nodes = make([]INetemNode, len(topology.Nodes))
	for idx, nConfig := range topology.Nodes {
		t.logger.Debugf("Create node %s", nConfig.Name)

		t.nodes[idx], err = CreateNode(t.prjID, nConfig)
		if err != nil {
			return fmt.Errorf("Unable to create node %s: %w", nConfig.Name, err)
		}
	}

	// Create links
	t.links = make([]*NetemLink, len(topology.Links))
	for idx, lConfig := range topology.Links {
		peer1 := strings.Split(lConfig.Peer1, ".")
		peer2 := strings.Split(lConfig.Peer2, ".")

		peer1Idx, _ := strconv.Atoi(peer1[1])
		peer2Idx, _ := strconv.Atoi(peer2[1])

		t.links[idx] = &NetemLink{
			Peer1: NetemLinkPeer{
				Node:    t.GetNode(peer1[0]),
				IfIndex: peer1Idx,
			},
			Peer2: NetemLinkPeer{
				Node:    t.GetNode(peer2[0]),
				IfIndex: peer2Idx,
			},
		}
	}

	// Create bridges
	t.bridges = make([]*NetemBridge, len(topology.Bridges))
	for idx, bConfig := range topology.Bridges {
		t.bridges[idx] = &NetemBridge{
			Name:          options.NETEM_ID + t.prjID + "." + shortName(bConfig.Name),
			HostInterface: bConfig.Host,
			Peers:         make([]NetemLinkPeer, len(bConfig.Interfaces)),
		}

		for pIdx, ifName := range bConfig.Interfaces {
			peer := strings.Split(ifName, ".")
			peerIdx, _ := strconv.Atoi(peer[1])

			t.bridges[idx].Peers[pIdx] = NetemLinkPeer{
				Node:    t.GetNode(peer[0]),
				IfIndex: peerIdx,
			}
		}
	}

	return nil
}

func (t *NetemTopologyManager) Reload() error {
	if err := t.Close(); err != nil {
		return err
	}

	if err := t.Load(); err != nil {
		return err
	}
	if t.running {
		t.running = false
		return t.Run()
	}

	return nil
}

func (t *NetemTopologyManager) Run() error {
	t.logger.Debug("Topo/Run")

	var err error
	if t.running {
		t.logger.Warn("Topology is already running")
		return nil
	}

	g := new(errgroup.Group)
	// 1 - start ovswitch container and init p2pSwitch
	t.logger.Debug("Topo/Run: start ovswitch instance")
	t.ovsInstance.Start()
	if err != nil {
		return err
	}

	// 2 - start all nodes
	t.logger.Debug("Topo/Run: start all nodes")
	for _, node := range t.nodes {
		node := node
		g.Go(func() error { return node.Start() })
	}
	if err := g.Wait(); err != nil {
		return err
	}

	// 3 - create links
	t.logger.Debug("Topo/Run: setup links")
	for _, l := range t.links {
		l := l
		g.Go(func() error {
			return t.setupLink(l)
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	// 4 - create bridges
	t.logger.Debug("Topo/Run: setup bridges")
	for _, br := range t.bridges {
		br := br
		g.Go(func() error {
			return t.setupBridge(br)
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	// 5 - load configs
	t.logger.Debug("Topo/Run: load configuration")
	configPath := path.Join(t.path, configDir)
	for _, node := range t.nodes {
		node := node
		g.Go(func() error { return node.LoadConfig(configPath) })
	}
	if err := g.Wait(); err != nil {
		return err
	}

	t.running = true
	return nil
}

func (t *NetemTopologyManager) setupBridge(br *NetemBridge) error {
	rootNs := link.GetRootNetns()
	defer rootNs.Close()

	brId, err := link.CreateBridge(br.Name, rootNs)
	if err != nil {
		return err
	}

	if err := link.AttachToBridge(brId, br.HostInterface, rootNs); err != nil {
		return fmt.Errorf("Unable to attach HostIf to bridge %s: %v", br.Name, err)
	}

	for _, peer := range br.Peers {
		peerNetns, err := peer.Node.GetNetns()
		if err != nil {
			return err
		}
		defer peerNetns.Close()

		ifName := fmt.Sprintf("%s%s%s.%d", options.NETEM_ID, t.prjID, shortName(peer.Node.GetName()), peer.IfIndex)
		veth, err := link.CreateVethLink(
			ifName, rootNs,
			peer.Node.GetInterfaceName(peer.IfIndex), peerNetns,
		)
		if err != nil {
			return fmt.Errorf(
				"Unable to create link %s-%s.%d: %v",
				br.Name, peer.Node.GetName(), peer.IfIndex, err,
			)
		}

		// set interface up
		if err := link.SetInterfaceState(veth.Name, rootNs, link.IFSTATE_UP); err != nil {
			return err
		}
		if err := link.SetInterfaceState(veth.PeerName, peerNetns, link.IFSTATE_UP); err != nil {
			return err
		}

		if err := link.AttachToBridge(brId, veth.Name, rootNs); err != nil {
			return err
		}
		peer.Node.AddInterface(peer.IfIndex)
	}

	return nil
}

func (t *NetemTopologyManager) setupLink(l *NetemLink) error {
	peer1Netns, err := l.Peer1.Node.GetNetns()
	if err != nil {
		return err
	}
	defer peer1Netns.Close()

	peer2Netns, err := l.Peer2.Node.GetNetns()
	if err != nil {
		return err
	}
	defer peer2Netns.Close()

	veth, err := link.CreateVethLink(
		l.Peer1.Node.GetInterfaceName(l.Peer1.IfIndex), peer1Netns,
		l.Peer2.Node.GetInterfaceName(l.Peer2.IfIndex), peer2Netns,
	)
	if err != nil {
		return fmt.Errorf(
			"Unable to create link %s.%d-%s.%d: %v",
			l.Peer1.Node.GetName(), l.Peer1.IfIndex,
			l.Peer2.Node.GetName(), l.Peer2.IfIndex,
			err,
		)
	}

	// set interface up
	if err := link.SetInterfaceState(veth.Name, peer1Netns, link.IFSTATE_UP); err != nil {
		return err
	}
	if err := link.SetInterfaceState(veth.PeerName, peer2Netns, link.IFSTATE_UP); err != nil {
		return err
	}

	// record interface in node
	l.Peer1.Node.AddInterface(l.Peer1.IfIndex)
	l.Peer2.Node.AddInterface(l.Peer2.IfIndex)

	return nil
}

func (t *NetemTopologyManager) IsRunning() bool {
	return t.running
}

func (t *NetemTopologyManager) GetNetFilePath() string {
	return path.Join(t.path, networkFilename)
}

func (t *NetemTopologyManager) ReadNetworkFile() ([]byte, error) {
	return ioutil.ReadFile(t.GetNetFilePath())
}

func (t *NetemTopologyManager) WriteNetworkFile(data []byte) error {
	return ioutil.WriteFile(t.GetNetFilePath(), data, 0644)
}

func (t *NetemTopologyManager) GetAllNodes() []INetemNode {
	return t.nodes
}

func (t *NetemTopologyManager) GetNode(name string) INetemNode {
	for _, node := range t.nodes {
		if node.GetName() == name {
			return node
		}
	}
	return nil
}

func (t *NetemTopologyManager) startNode(node INetemNode) error {
	if err := node.Start(); err != nil {
		return fmt.Errorf("Unable to start node %s: %w", node.GetName(), err)
	}

	configPath := path.Join(t.path, configDir)
	if err := node.LoadConfig(configPath); err != nil {
		return fmt.Errorf("Unable to load config of node %s: %w", node.GetName(), err)
	}

	return nil
}

func (t *NetemTopologyManager) stopNode(node INetemNode) error {
	if err := node.Stop(); err != nil {
		return fmt.Errorf("Unable to stop node %s: %w", node.GetName(), err)
	}
	return nil
}

func (t *NetemTopologyManager) Start(nodeName string) error {
	if !t.running {
		t.logger.Warnf("Start %s: topology not running", nodeName)
		return nil
	}

	node := t.GetNode(nodeName)
	if node == nil {
		return fmt.Errorf("Node %s not found in the topology", nodeName)
	}

	return t.startNode(node)
}

func (t *NetemTopologyManager) Stop(nodeName string) error {
	if !t.running {
		t.logger.Warnf("Stop %s: topology not running", nodeName)
		return nil
	}

	node := t.GetNode(nodeName)
	if node == nil {
		return fmt.Errorf("Node %s not found in the topology", nodeName)
	}

	return t.stopNode(node)
}

func (t *NetemTopologyManager) Save() error {
	// create config folder if not exist
	destPath := path.Join(t.path, configDir)
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		if err := os.Mkdir(destPath, 0755); err != nil {
			return fmt.Errorf("Unable to create configs dir %s: %w", destPath, err)
		}
	}

	g := new(errgroup.Group)
	for _, node := range t.nodes {
		node := node
		g.Go(func() error { return node.Save(destPath) })
	}
	return g.Wait()
}

func (t *NetemTopologyManager) Copy(source, dest string) error {
	var container INetemNode
	srcContainer, srcPath := splitCopyArg(source)
	destContainer, destPath := splitCopyArg(dest)

	var direction copyDirection
	if srcContainer != "" {
		direction |= fromContainer
		container = t.GetNode(srcContainer)
		if container == nil {
			return fmt.Errorf("Node %s not found in the topology", srcContainer)
		}
	}
	if destContainer != "" {
		direction |= toContainer
		container = t.GetNode(destContainer)
		if container == nil {
			return fmt.Errorf("Node %s not found in the topology", destContainer)
		}
	}

	if container.GetType() != "docker" {
		return errors.New("Selected node does not support copy")
	}
	dockerNode := container.(*docker.DockerNode)

	switch direction {
	case fromContainer:
		return dockerNode.CopyFrom(srcPath, destPath)
	case toContainer:
		return dockerNode.CopyTo(srcPath, destPath)
	case acrossContainers:
		return errors.New("copying between containers is not supported")
	default:
		return errors.New("must specify at least one container source")
	}
}

func (t *NetemTopologyManager) Close() error {
	for _, node := range t.nodes {
		if node != nil {
			if err := node.Close(); err != nil {
				t.logger.Errorf("Error when closing node %s: %v", node.GetName(), err)
			}
		}
	}

	rootNs := link.GetRootNetns()
	defer rootNs.Close()
	for _, br := range t.bridges {
		if err := link.DeleteLink(br.Name, rootNs); err != nil {
			t.logger.Warnf("Error when deleting bridge %s: %v", br.Name, err)
		}

		for _, peer := range br.Peers {
			ifName := fmt.Sprintf(
				"%s%s%s.%d", options.NETEM_ID, t.prjID,
				shortName(peer.Node.GetName()), peer.IfIndex)
			if err := link.DeleteLink(ifName, rootNs); err != nil {
				t.logger.Warnf("Error when deleting link %s: %v", ifName, err)
			}
		}
	}

	t.nodes = make([]INetemNode, 0)
	t.links = make([]*NetemLink, 0)
	t.bridges = make([]*NetemBridge, 0)

	if err := ovs.CloseOvsInstance(t.prjID); err != nil {
		t.logger.Warnf("Error when closing ovwitch instance: %v", err)
	}
	t.ovsInstance = nil

	return nil
}

func LoadTopology(prjID, prjPath string) (*NetemTopologyManager, error) {
	topo := &NetemTopologyManager{
		prjID:  prjID,
		path:   prjPath,
		nodes:  make([]INetemNode, 0),
		logger: logrus.WithField("project", prjID),
	}
	if err := topo.Load(); err != nil {
		return topo, fmt.Errorf("Unable to load the topology:\n\t%w", err)
	}
	return topo, nil
}
