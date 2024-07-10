package server

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/creasty/defaults"
	"github.com/mroy31/gonetem/internal/link"
	"github.com/mroy31/gonetem/internal/options"
	"github.com/mroy31/gonetem/internal/ovs"
	"github.com/mroy31/gonetem/internal/proto"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v2"
)

const (
	networkFilename       = "network.yml"
	configDir             = "configs"
	maxConcurrentNodeTask = 30
)

var (
	mutex = &sync.Mutex{}
)

type VrrpOptions struct {
	Interface int
	Group     int
	Address   string
}

type NodeConfig struct {
	Type    string
	IPv6    bool          `yaml:",omitempty" default:"false"`
	Mpls    bool          `yaml:",omitempty" default:"false"`
	Vrfs    []string      `yaml:",omitempty"`
	Vrrps   []VrrpOptions `yaml:",omitempty"`
	Volumes []string      `yaml:",omitempty"`
	Image   string        `yaml:",omitempty"`
	Launch  bool          `default:"true"`
}

type RunCloseProgressCode int
type SaveProgressCode int
type CloseProgressCode int

const (
	NODE_COUNT      RunCloseProgressCode = 1
	BRIDGE_COUNT    RunCloseProgressCode = 2
	LINK_COUNT      RunCloseProgressCode = 3
	LOAD_TOPO       RunCloseProgressCode = 4
	START_NODE      RunCloseProgressCode = 5
	SETUP_LINK      RunCloseProgressCode = 6
	START_BRIDGE    RunCloseProgressCode = 7
	LOADCONFIG_NODE RunCloseProgressCode = 8
	CLOSE_NODE      RunCloseProgressCode = 9
	CLOSE_BRIDGE    RunCloseProgressCode = 10
)

type TopologyRunCloseProgressT struct {
	Code  RunCloseProgressCode
	Value int
}

const (
	NODE_SAVE_COUNT SaveProgressCode = 1
	NODE_SAVE       SaveProgressCode = 2
)

type TopologySaveProgressT struct {
	Code  SaveProgressCode
	Value int
}

func (n *NodeConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	defaults.Set(n)

	type plain NodeConfig
	if err := unmarshal((*plain)(n)); err != nil {
		return err
	}

	return nil
}

type LinkConfig struct {
	Peer1  string
	Peer2  string
	Loss   float64 `yaml:",omitempty"` // percent
	Delay  int     `yaml:",omitempty"` // ms
	Jitter int     `yaml:",omitempty"` // ms
	Rate   int     `yaml:",omitempty"` // kbps
	Buffer float64 `yaml:",omitempty"` // BDP scale factor
}

type BridgeConfig struct {
	Host       string
	Interfaces []string `yaml:",omitempty"`
}

type NetemTopology struct {
	Nodes   map[string]NodeConfig   `yaml:",omitempty"`
	Links   []LinkConfig            `yaml:",omitempty"`
	Bridges map[string]BridgeConfig `yaml:",omitempty"`
}

type NetemLinkPeer struct {
	Node    INetemNode
	IfIndex int
}

type NetemLink struct {
	Peer1    NetemLinkPeer
	Peer2    NetemLinkPeer
	Config   LinkConfig
	HasNetem bool
	HasTbf   bool
}

type NetemBridge struct {
	Name          string
	HostInterface string
	Peers         []NetemLinkPeer
	Config        BridgeConfig
}

type NetemNode struct {
	Instance        INetemNode
	LaunchAtStartup bool
	Config          NodeConfig
}

type NetemTopologyManager struct {
	prjID string
	path  string

	IdGenerator *NodeIdentifierGenerator
	nodes       []NetemNode
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
		return fmt.Errorf("topology is not valid:%s", msg)
	}

	return nil
}

func (t *NetemTopologyManager) SynchroniseTopology() error {
	topo := &NetemTopology{
		Nodes: make(map[string]NodeConfig),
		Links: make([]LinkConfig, 0),
	}

	for _, node := range t.nodes {
		topo.Nodes[node.Instance.GetName()] = node.Config
	}

	for _, link := range t.links {
		topo.Links = append(topo.Links, link.Config)
	}

	data, err := yaml.Marshal(topo)
	if err != nil {
		return fmt.Errorf("unable to marshal yaml topo: %v", err)
	}

	if err := os.WriteFile(t.GetNetFilePath(), data, 0644); err != nil {
		return fmt.Errorf("unable to write network file: %v", err)
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
		return fmt.Errorf("topology if not valid:%s", msg)
	}

	var err error
	// Create openvswitch instance for this project
	t.ovsInstance, err = ovs.NewOvsInstance(t.prjID)
	if err != nil {
		return err
	}

	// Create nodes
	t.nodes = make([]NetemNode, 0)
	g := new(errgroup.Group)
	g.SetLimit(maxConcurrentNodeTask)

	for name, nConfig := range topology.Nodes {
		name := name
		nConfig := nConfig

		g.Go(func() error {
			t.logger.Debugf("Create node %s", name)

			shortName, err := t.IdGenerator.GetId(name)
			if err != nil {
				return err
			}
			node, err := CreateNode(t.prjID, name, shortName, nConfig)

			mutex.Lock()
			t.nodes = append(t.nodes, NetemNode{
				Instance:        node,
				LaunchAtStartup: nConfig.Launch,
				Config:          nConfig,
			})
			mutex.Unlock()

			if err != nil {
				return fmt.Errorf("unable to create node %s: %w", name, err)
			}

			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	// Create links
	t.links = make([]*NetemLink, len(topology.Links))
	for idx, lConfig := range topology.Links {
		peer1 := strings.Split(lConfig.Peer1, ".")
		peer2 := strings.Split(lConfig.Peer2, ".")

		peer1Idx, _ := strconv.Atoi(peer1[1])
		peer2Idx, _ := strconv.Atoi(peer2[1])

		if lConfig.Buffer == 0.0 {
			// by default set limit buffer to 1.0 * BDP
			lConfig.Buffer = 1.0
		}

		t.links[idx] = &NetemLink{
			Peer1: NetemLinkPeer{
				Node:    t.GetNode(peer1[0]),
				IfIndex: peer1Idx,
			},
			Peer2: NetemLinkPeer{
				Node:    t.GetNode(peer2[0]),
				IfIndex: peer2Idx,
			},
			HasNetem: false,
			HasTbf:   false,
			Config:   lConfig,
		}
	}

	// Create bridges
	bIdx := 0
	t.bridges = make([]*NetemBridge, len(topology.Bridges))
	for bName, bConfig := range topology.Bridges {
		shortName, err := t.IdGenerator.GetId(bName)
		if err != nil {
			return err
		}

		t.bridges[bIdx] = &NetemBridge{
			Name:          options.NETEM_ID + t.prjID + "." + shortName,
			HostInterface: bConfig.Host,
			Peers:         make([]NetemLinkPeer, len(bConfig.Interfaces)),
			Config:        bConfig,
		}

		for pIdx, ifName := range bConfig.Interfaces {
			peer := strings.Split(ifName, ".")
			peerIdx, _ := strconv.Atoi(peer[1])

			t.bridges[bIdx].Peers[pIdx] = NetemLinkPeer{
				Node:    t.GetNode(peer[0]),
				IfIndex: peerIdx,
			}
		}

		bIdx++
	}

	return nil
}

func (t *NetemTopologyManager) Reload(progressCh chan TopologyRunCloseProgressT) ([]*proto.TopologyRunMsg_NodeMessages, error) {
	t.logger.Debug("Topo/Reload")

	var err error
	var nodeMessages []*proto.TopologyRunMsg_NodeMessages

	if err = t.Close(progressCh); err != nil {
		return nodeMessages, err
	}

	if err = t.Load(); err != nil {
		return nodeMessages, err
	}

	if t.running {
		t.running = false
		return t.Run(progressCh)
	}

	return nodeMessages, nil
}

func (t *NetemTopologyManager) Run(progressCh chan TopologyRunCloseProgressT) ([]*proto.TopologyRunMsg_NodeMessages, error) {
	t.logger.Debug("Topo/Run")
	if progressCh != nil {
		progressCh <- TopologyRunCloseProgressT{Code: NODE_COUNT, Value: len(t.nodes)}
		progressCh <- TopologyRunCloseProgressT{Code: BRIDGE_COUNT, Value: len(t.bridges)}
		progressCh <- TopologyRunCloseProgressT{Code: LINK_COUNT, Value: len(t.links)}
	}

	var err error
	var nodeMessages []*proto.TopologyRunMsg_NodeMessages

	if t.running {
		t.logger.Warn("Topology is already running")
		return nodeMessages, nil
	}

	// 1 - start ovswitch container and init p2pSwitch
	t.logger.Debug("Topo/Run: start ovswitch instance")
	err = t.ovsInstance.Start()
	if err != nil {
		return nodeMessages, err
	}

	// 2 - start all required nodes
	g := new(errgroup.Group)
	g.SetLimit(maxConcurrentNodeTask)

	t.logger.Debug("Topo/Run: start nodes")
	for _, node := range t.nodes {
		node := node
		g.Go(func() error {
			var err error = nil

			if node.LaunchAtStartup {
				err = node.Instance.Start()
			}

			if progressCh != nil {
				progressCh <- TopologyRunCloseProgressT{Code: START_NODE}
			}
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return nodeMessages, err
	}

	// 3 - create links
	t.logger.Debug("Topo/Run: setup links")
	for _, l := range t.links {
		if err := t.setupLink(l); err != nil {
			return nodeMessages, err
		}

		if progressCh != nil {
			progressCh <- TopologyRunCloseProgressT{Code: SETUP_LINK}
		}
	}

	// 4 - create bridges
	t.logger.Debug("Topo/Run: setup bridges")
	for _, br := range t.bridges {
		br := br
		g.Go(func() error {
			err := t.setupBridge(br)

			if progressCh != nil {
				progressCh <- TopologyRunCloseProgressT{Code: START_BRIDGE}
			}
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return nodeMessages, err
	}

	// 5 - load configs
	t.logger.Debug("Topo/Run: load configuration")
	timeout := options.ServerConfig.Docker.Timeoutop
	configPath := path.Join(t.path, configDir)
	for _, node := range t.nodes {
		node := node
		g.Go(func() error {
			var messages []string
			var err error = nil

			if node.LaunchAtStartup {
				messages, err = node.Instance.LoadConfig(configPath, timeout)
				nodeMessages = append(nodeMessages, &proto.TopologyRunMsg_NodeMessages{
					Name:     node.Instance.GetName(),
					Messages: messages,
				})
			}

			if progressCh != nil {
				progressCh <- TopologyRunCloseProgressT{Code: LOADCONFIG_NODE}
			}
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return nodeMessages, err
	}

	t.running = true
	return nodeMessages, nil
}

func (t *NetemTopologyManager) setupBridge(br *NetemBridge) error {
	rootNs := link.GetRootNetns()
	defer rootNs.Close()

	brId, err := link.CreateBridge(br.Name, rootNs)
	if err != nil {
		return err
	}

	if err := link.AttachToBridge(brId, br.HostInterface, rootNs); err != nil {
		return fmt.Errorf("unable to attach HostIf to bridge %s: %v", br.Name, err)
	}

	for _, peer := range br.Peers {
		peerNetns, err := peer.Node.GetNetns()
		if err != nil {
			return err
		}
		defer peerNetns.Close()

		ifName := fmt.Sprintf("%s%s%s.%d", options.NETEM_ID, t.prjID, peer.Node.GetShortName(), peer.IfIndex)
		peerIfName := fmt.Sprintf("%s%s%d.%s", options.NETEM_ID, t.prjID, peer.IfIndex, peer.Node.GetShortName())
		veth, err := link.CreateVethLink(
			ifName, rootNs,
			peerIfName, peerNetns,
		)
		if err != nil {
			return fmt.Errorf(
				"unable to create link %s-%s.%d: %v",
				br.Name, peer.Node.GetName(), peer.IfIndex, err,
			)
		}

		// set interface up
		if err := link.SetInterfaceState(veth.Name, rootNs, link.IFSTATE_UP); err != nil {
			return err
		}

		if err := link.AttachToBridge(brId, veth.Name, rootNs); err != nil {
			return err
		}
		peer.Node.AddInterface(peerIfName, peer.IfIndex, peerNetns)
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

	peer1IfName := fmt.Sprintf("%s%s.%d", t.prjID, l.Peer1.Node.GetShortName(), l.Peer1.IfIndex)
	peer2IfName := fmt.Sprintf("%s%s.%d", t.prjID, l.Peer2.Node.GetShortName(), l.Peer2.IfIndex)
	_, err = link.CreateVethLink(peer1IfName, peer1Netns, peer2IfName, peer2Netns)
	if err != nil {
		return fmt.Errorf(
			"unable to create link %s.%d-%s.%d: %v",
			l.Peer1.Node.GetName(), l.Peer1.IfIndex,
			l.Peer2.Node.GetName(), l.Peer2.IfIndex,
			err,
		)
	}

	// create netem qdisc if necessary
	if l.Config.Delay > 0 || l.Config.Loss > 0 {
		if err := link.Netem(peer1IfName, peer1Netns, l.Config.Delay, l.Config.Jitter, l.Config.Loss, false); err != nil {
			return err
		}
		if err := link.Netem(peer2IfName, peer2Netns, l.Config.Delay, l.Config.Jitter, l.Config.Loss, false); err != nil {
			return err
		}
		l.HasNetem = true
	}
	// create tbf qdisc if necessary
	if l.Config.Rate > 0 {
		if err := link.CreateTbf(peer1IfName, peer1Netns, l.Config.Delay+l.Config.Jitter, l.Config.Rate, l.Config.Buffer); err != nil {
			return err
		}
		if err := link.CreateTbf(peer2IfName, peer2Netns, l.Config.Delay+l.Config.Jitter, l.Config.Rate, l.Config.Buffer); err != nil {
			return err
		}
		l.HasTbf = true
	}

	if err := l.Peer1.Node.AddInterface(peer1IfName, l.Peer1.IfIndex, peer1Netns); err != nil {
		return err
	}
	if err := l.Peer2.Node.AddInterface(peer2IfName, l.Peer2.IfIndex, peer2Netns); err != nil {
		return err
	}

	return nil
}

func (t *NetemTopologyManager) GetLink(peer1V string, peer2V string) (*NetemLink, int, error) {
	peer1 := strings.Split(peer1V, ".")
	peer2 := strings.Split(peer2V, ".")

	peer1Idx, _ := strconv.Atoi(peer1[1])
	peer2Idx, _ := strconv.Atoi(peer2[1])

	for idx, l := range t.links {
		if l.Peer1.IfIndex == peer1Idx &&
			l.Peer1.Node.GetName() == peer1[0] &&
			l.Peer2.IfIndex == peer2Idx &&
			l.Peer2.Node.GetName() == peer2[0] {
			return l, idx, nil
		}
	}

	// check for inverse link
	for idx, l := range t.links {
		if l.Peer1.IfIndex == peer2Idx &&
			l.Peer1.Node.GetName() == peer2[0] &&
			l.Peer2.IfIndex == peer1Idx &&
			l.Peer2.Node.GetName() == peer1[0] {
			return l, idx, nil
		}
	}

	return nil, -1, fmt.Errorf(
		"link %s - %s not found in the topology",
		peer1V, peer2V,
	)
}

func (t *NetemTopologyManager) LinkAdd(linkCfg LinkConfig, sync bool) error {
	_, _, err := t.GetLink(linkCfg.Peer1, linkCfg.Peer2)
	if err == nil {
		return fmt.Errorf("this link already exist")
	}

	peer1 := strings.Split(linkCfg.Peer1, ".")
	peer2 := strings.Split(linkCfg.Peer2, ".")

	peer1Idx, _ := strconv.Atoi(peer1[1])
	peer2Idx, _ := strconv.Atoi(peer2[1])

	if linkCfg.Buffer == 0.0 {
		// by default set limit buffer to 1.0 * BDP
		linkCfg.Buffer = 1.0
	}

	link := &NetemLink{
		Peer1: NetemLinkPeer{
			Node:    t.GetNode(peer1[0]),
			IfIndex: peer1Idx,
		},
		Peer2: NetemLinkPeer{
			Node:    t.GetNode(peer2[0]),
			IfIndex: peer2Idx,
		},
		HasNetem: false,
		HasTbf:   false,
		Config:   linkCfg,
	}
	if err := t.setupLink(link); err != nil {
		return err
	}

	t.links = append(t.links, link)
	if sync {
		return t.SynchroniseTopology()
	}

	return nil
}

func (t *NetemTopologyManager) LinkDel(linkCfg LinkConfig, sync bool) error {
	l, idx, err := t.GetLink(linkCfg.Peer1, linkCfg.Peer2)
	if err != nil {
		return err
	}

	peer1Netns, err := l.Peer1.Node.GetNetns()
	if err != nil {
		return err
	}
	defer peer1Netns.Close()

	peer1IfName := l.Peer1.Node.GetInterfaceName(l.Peer1.IfIndex)
	if err := link.DeleteLink(peer1IfName, peer1Netns); err != nil {
		return err
	}

	t.links = append(t.links[:idx], t.links[idx+1:]...)
	if sync {
		return t.SynchroniseTopology()
	}
	return nil
}

func (t *NetemTopologyManager) LinkUpdate(linkCfg LinkConfig, sync bool) error {
	l, _, err := t.GetLink(linkCfg.Peer1, linkCfg.Peer2)
	if err != nil {
		return err
	}
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

	peer1IfName := l.Peer1.Node.GetInterfaceName(l.Peer1.IfIndex)
	peer2IfName := l.Peer2.Node.GetInterfaceName(l.Peer2.IfIndex)
	if err := link.Netem(peer1IfName, peer1Netns, linkCfg.Delay, linkCfg.Jitter, linkCfg.Loss, l.HasNetem); err != nil {
		return err
	}
	if err := link.Netem(peer2IfName, peer2Netns, linkCfg.Delay, linkCfg.Jitter, linkCfg.Loss, l.HasNetem); err != nil {
		return err
	}
	l.HasNetem = true

	// update config
	l.Config.Delay = linkCfg.Delay
	l.Config.Jitter = linkCfg.Jitter
	l.Config.Loss = linkCfg.Loss

	if sync {
		return t.SynchroniseTopology()
	}
	return nil
}

func (t *NetemTopologyManager) IsRunning() bool {
	return t.running
}

func (t *NetemTopologyManager) GetNetFilePath() string {
	return path.Join(t.path, networkFilename)
}

func (t *NetemTopologyManager) ReadNetworkFile() ([]byte, error) {
	return os.ReadFile(t.GetNetFilePath())
}

func (t *NetemTopologyManager) WriteNetworkFile(data []byte) error {
	return os.WriteFile(t.GetNetFilePath(), data, 0644)
}

func (t *NetemTopologyManager) GetAllNodes() []INetemNode {
	nodeInstances := make([]INetemNode, len(t.nodes))
	for i := range t.nodes {
		nodeInstances[i] = t.nodes[i].Instance
	}
	return nodeInstances
}

func (t *NetemTopologyManager) IsNodeLaunchAtStartup(name string) bool {
	for _, node := range t.nodes {
		if node.Instance.GetName() == name {
			return node.LaunchAtStartup
		}
	}
	return false
}

func (t *NetemTopologyManager) GetNode(name string) INetemNode {
	for _, node := range t.nodes {
		if node.Instance.GetName() == name {
			return node.Instance
		}
	}
	return nil
}

func (t *NetemTopologyManager) startNode(node INetemNode) ([]string, error) {
	if err := node.Start(); err != nil {
		return []string{}, fmt.Errorf("unable to start node %s: %w", node.GetName(), err)
	}

	configPath := path.Join(t.path, configDir)
	timeout := options.ServerConfig.Docker.Timeoutop
	messages, err := node.LoadConfig(configPath, timeout)
	if err != nil {
		return messages, fmt.Errorf("unable to load config of node %s: %w", node.GetName(), err)
	}

	return messages, nil
}

func (t *NetemTopologyManager) stopNode(node INetemNode) error {
	if err := node.Stop(); err != nil {
		return fmt.Errorf("unable to stop node %s: %w", node.GetName(), err)
	}
	return nil
}

func (t *NetemTopologyManager) Start(nodeName string) ([]string, error) {
	if !t.running {
		t.logger.Warnf("Start %s: topology not running", nodeName)
		return []string{}, nil
	}

	node := t.GetNode(nodeName)
	if node == nil {
		return []string{}, fmt.Errorf("node %s not found in the topology", nodeName)
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
		return fmt.Errorf("node %s not found in the topology", nodeName)
	}

	return t.stopNode(node)
}

func (t *NetemTopologyManager) ReadConfigFiles(nodeName string) (map[string][]byte, error) {
	node := t.GetNode(nodeName)
	if node == nil {
		return map[string][]byte{}, fmt.Errorf("node %s not found in the topology", nodeName)
	}

	confPath := path.Join(t.path, configDir)
	timeout := options.ServerConfig.Docker.Timeoutop

	return node.ReadConfigFiles(confPath, timeout)
}

func (t *NetemTopologyManager) Save(progressCh chan TopologySaveProgressT) error {
	// create config folder if not exist
	destPath := path.Join(t.path, configDir)
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		if err := os.Mkdir(destPath, 0755); err != nil {
			return fmt.Errorf("unable to create configs dir %s: %w", destPath, err)
		}
	}

	if progressCh != nil {
		progressCh <- TopologySaveProgressT{Code: NODE_SAVE_COUNT, Value: len(t.nodes)}
	}

	timeout := options.ServerConfig.Docker.Timeoutop
	g := new(errgroup.Group)
	g.SetLimit(maxConcurrentNodeTask)

	for _, node := range t.nodes {
		node := node

		g.Go(func() error {
			var err error = nil

			if node.Instance.IsRunning() {
				err = node.Instance.Save(destPath, timeout)
			}

			if progressCh != nil {
				progressCh <- TopologySaveProgressT{Code: NODE_SAVE}
			}

			if err != nil {
				return fmt.Errorf("node %s: save cmd error - %v", node.Instance.GetName(), err)
			}
			return nil
		})
	}
	err := g.Wait()

	return err
}

func (t *NetemTopologyManager) Close(progressCh chan TopologyRunCloseProgressT) error {
	if progressCh != nil {
		progressCh <- TopologyRunCloseProgressT{Code: NODE_COUNT, Value: len(t.nodes)}
		progressCh <- TopologyRunCloseProgressT{Code: BRIDGE_COUNT, Value: len(t.bridges)}
	}

	g := new(errgroup.Group)
	g.SetLimit(maxConcurrentNodeTask)

	// close all nodes
	for _, node := range t.nodes {
		node := node
		g.Go(func() error {
			err := node.Instance.Close()
			if progressCh != nil {
				progressCh <- TopologyRunCloseProgressT{Code: CLOSE_NODE}
			}

			if err != nil {
				return fmt.Errorf("node %s: %v", node.Instance.GetName(), err)
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		t.logger.Errorf("Error when closing nodes: %v", err)
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
				peer.Node.GetShortName(), peer.IfIndex)
			if err := link.DeleteLink(ifName, rootNs); err != nil {
				t.logger.Warnf("Error when deleting link %s: %v", ifName, err)
			}

			if progressCh != nil {
				progressCh <- TopologyRunCloseProgressT{Code: CLOSE_BRIDGE}
			}
		}
	}

	t.nodes = make([]NetemNode, 0)
	t.links = make([]*NetemLink, 0)
	t.bridges = make([]*NetemBridge, 0)
	t.IdGenerator.Close()

	if err := ovs.CloseOvsInstance(t.prjID); err != nil {
		t.logger.Warnf("Error when closing ovswitch instance: %v", err)
	}
	t.ovsInstance = nil

	return nil
}

func LoadTopology(prjID, prjPath string) (*NetemTopologyManager, error) {
	topo := &NetemTopologyManager{
		prjID:  prjID,
		path:   prjPath,
		nodes:  make([]NetemNode, 0),
		logger: logrus.WithField("project", prjID),
		IdGenerator: &NodeIdentifierGenerator{
			lock: &sync.Mutex{},
		},
	}
	if err := topo.Load(); err != nil {
		return topo, fmt.Errorf("unable to load the topology:\n\t%w", err)
	}
	return topo, nil
}
