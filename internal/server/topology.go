package server

import (
	"fmt"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/creasty/defaults"
	"github.com/mroy31/gonetem/internal/link"
	"github.com/mroy31/gonetem/internal/options"
	"github.com/mroy31/gonetem/internal/ovs"
	"github.com/mroy31/gonetem/internal/proto"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netns"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
)

const (
	networkFilename       = "network.yml"
	configDir             = "configs"
	maxConcurrentNodeTask = 30
)

var (
	mutex = &sync.Mutex{}
)

type QoSConfig struct {
	Loss   float64 `yaml:",omitempty"` // percent
	Delay  int     `yaml:",omitempty"` // ms
	Jitter int     `yaml:",omitempty"` // ms
	Rate   int     `yaml:",omitempty"` // kbps
	Buffer float64 `yaml:",omitempty"` // BDP scale factor
}

type LinkConfig struct {
	Peer1    string
	Peer2    string
	Loss     float64 `yaml:",omitempty"` // percent
	Delay    int     `yaml:",omitempty"` // ms
	Jitter   int     `yaml:",omitempty"` // ms
	Rate     int     `yaml:",omitempty"` // kbps
	Buffer   float64 `yaml:",omitempty"` // BDP scale factor
	Peer1QoS QoSConfig
	Peer2QoS QoSConfig
}

func (l *LinkConfig) GetPeer1QoS() QoSConfig {
	if l.Peer1QoS.Loss > 0 ||
		l.Peer1QoS.Delay > 0 ||
		l.Peer1QoS.Jitter > 0 ||
		l.Peer1QoS.Rate > 0 ||
		l.Peer1QoS.Buffer > 0 {
		return l.Peer1QoS
	}

	return QoSConfig{
		Loss:   l.Loss,
		Delay:  l.Delay,
		Jitter: l.Jitter,
		Rate:   l.Rate,
		Buffer: l.Buffer,
	}
}

func (l *LinkConfig) GetPeer2QoS() QoSConfig {
	if l.Peer2QoS.Loss > 0 ||
		l.Peer2QoS.Delay > 0 ||
		l.Peer2QoS.Jitter > 0 ||
		l.Peer2QoS.Rate > 0 ||
		l.Peer2QoS.Buffer > 0 {
		return l.Peer2QoS
	}

	return QoSConfig{
		Loss:   l.Loss,
		Delay:  l.Delay,
		Jitter: l.Jitter,
		Rate:   l.Rate,
		Buffer: l.Buffer,
	}
}

type BridgeConfig struct {
	Host       string
	Interfaces []string `yaml:",omitempty"`
}

type MgntNetworkConfig struct {
	Enable  bool   `yaml:",omitempty" default:"false"`
	Address string `yaml:",omitempty"`
}
type VrrpOptions struct {
	Interface int
	Group     int
	Address   string
}

type MgntOptions struct {
	Enable  bool   `yaml:",omitempty" default:"false"`
	Address string `yaml:",omitempty"`
}

type NodeConfig struct {
	Type    string
	IPv6    bool          `yaml:",omitempty" default:"true"`
	Mpls    bool          `yaml:",omitempty" default:"false"`
	Vrfs    []string      `yaml:",omitempty"`
	Vrrps   []VrrpOptions `yaml:",omitempty"`
	Volumes []string      `yaml:",omitempty"`
	Image   string        `yaml:",omitempty"`
	Launch  bool          `default:"true"`
	Mgnt    MgntOptions   `yaml:",omitempty"`
}

func (n *NodeConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	defaults.Set(n)

	type plain NodeConfig
	if err := unmarshal((*plain)(n)); err != nil {
		return err
	}

	return nil
}

type RunCloseProgressCode int
type SaveProgressCode int
type CloseProgressCode int

const (
	NODE_COUNT      RunCloseProgressCode = 1
	BRIDGE_COUNT    RunCloseProgressCode = 3
	LINK_COUNT      RunCloseProgressCode = 4
	LOAD_TOPO       RunCloseProgressCode = 5
	START_NODE      RunCloseProgressCode = 6
	SETUP_LINK      RunCloseProgressCode = 7
	START_BRIDGE    RunCloseProgressCode = 8
	LOADCONFIG_NODE RunCloseProgressCode = 10
	CLOSE_NODE      RunCloseProgressCode = 11
	CLOSE_BRIDGE    RunCloseProgressCode = 12
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

type NetemTopology struct {
	Nodes   map[string]NodeConfig   `yaml:",omitempty"`
	Links   []LinkConfig            `yaml:",omitempty"`
	Bridges map[string]BridgeConfig `yaml:",omitempty"`
	Mgntnet MgntNetworkConfig       `yaml:",omitempty"`
}

type NetemLinkPeer struct {
	Node    INetemNode
	IfIndex int
}

type NetemLink struct {
	Peer1         NetemLinkPeer
	Peer2         NetemLinkPeer
	Config        LinkConfig
	HasPeer1Netem bool
	HasPeer2Netem bool
	HasPeer1Tbf   bool
	HasPeer2Tbf   bool
}

func (l *NetemLink) IsNetemRequired(q QoSConfig) bool {
	return q.Delay > 0 || q.Loss > 0 || q.Jitter > 0
}

func (l *NetemLink) SetPeer1TBF(ifName string, ns netns.NsHandle) error {
	peerQoS := l.Config.GetPeer1QoS()

	// create tbf qdisc if necessary
	if peerQoS.Rate > 0 {
		if err := link.CreateTbf(ifName, ns, peerQoS.Delay+l.Config.Jitter, peerQoS.Rate, peerQoS.Buffer, l.HasPeer1Tbf); err != nil {
			return err
		}
		l.HasPeer1Tbf = true
	}

	return nil
}

func (l *NetemLink) SetPeer2TBF(ifName string, ns netns.NsHandle) error {
	peerQoS := l.Config.GetPeer2QoS()

	// create tbf qdisc if necessary
	if peerQoS.Rate > 0 {
		if err := link.CreateTbf(ifName, ns, peerQoS.Delay+l.Config.Jitter, peerQoS.Rate, peerQoS.Buffer, l.HasPeer2Tbf); err != nil {
			return err
		}
		l.HasPeer2Tbf = true
	}

	return nil
}

func (l *NetemLink) SetPeer1Netem(ifName string, ns netns.NsHandle) error {
	peerQoS := l.Config.GetPeer1QoS()

	// create netem qdisc if necessary
	if l.IsNetemRequired(peerQoS) {
		if err := link.Netem(ifName, ns, peerQoS.Delay, peerQoS.Jitter, peerQoS.Loss, l.HasPeer1Netem); err != nil {
			return err
		}
		l.HasPeer1Netem = true
	}

	return nil
}

func (l *NetemLink) SetPeer2Netem(ifName string, ns netns.NsHandle) error {
	peerQoS := l.Config.GetPeer2QoS()

	if l.IsNetemRequired(peerQoS) {
		if err := link.Netem(ifName, ns, peerQoS.Delay, peerQoS.Jitter, peerQoS.Loss, l.HasPeer2Netem); err != nil {
			return err
		}
		l.HasPeer2Netem = true
	}

	return nil
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
	mgntNet     *MgntNetwork
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
		Nodes:   make(map[string]NodeConfig),
		Links:   make([]LinkConfig, 0),
		Bridges: make(map[string]BridgeConfig),
	}

	for _, node := range t.nodes {
		topo.Nodes[node.Instance.GetName()] = node.Config
	}

	for _, link := range t.links {
		topo.Links = append(topo.Links, link.Config)
	}

	for _, bridge := range t.bridges {
		topo.Bridges[bridge.Name] = bridge.Config
	}

	if t.mgntNet != nil {
		topo.Mgntnet = MgntNetworkConfig{
			Enable:  true,
			Address: t.mgntNet.IPAddress,
		}
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

			if err != nil {
				if !reflect.ValueOf(node).IsNil() {
					t.logger.Infof("error node %t", node == nil)
					node.Close()
				}

				return fmt.Errorf("unable to create node %s: %w", name, err)
			}

			mutex.Lock()
			t.nodes = append(t.nodes, NetemNode{
				Instance:        node,
				LaunchAtStartup: nConfig.Launch,
				Config:          nConfig,
			})
			mutex.Unlock()

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
			HasPeer1Netem: false,
			HasPeer2Netem: false,
			HasPeer1Tbf:   false,
			HasPeer2Tbf:   false,
			Config:        lConfig,
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

	// create mgnt network if necessary
	t.mgntNet = nil
	if topology.Mgntnet.Enable {
		t.mgntNet = &MgntNetwork{
			NetId:     fmt.Sprintf("%s.mgnt", t.prjID),
			IPAddress: topology.Mgntnet.Address,
			NetNs:     link.GetRootNetns(),
			Logger:    t.logger,
		}
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

	// 2 - optionnal - create mgnt network
	if t.mgntNet != nil {
		if err := t.mgntNet.Create(); err != nil {
			return nodeMessages, err
		}
	}

	// 3 - start all required nodes
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

			if err == nil && node.Config.Mgnt.Enable {
				err = t.setupMgntLink(&node)
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
		if err := t.setupLink(l, false); err != nil {
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
		g.Go(func() error {
			var messages []string
			var err error = nil

			if node.Instance.IsRunning() {
				if err = node.Instance.ConfigureInterfaces(); err != nil {
					return err
				}

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

		ifName := fmt.Sprintf("%s%s%s.%d", options.NETEM_ID, t.prjID, peer.Node.GetShortName(), peer.IfIndex)
		peerIfName := peer.Node.GetInterfaceName(peer.IfIndex)
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

		if err := peer.Node.AttachInterface(peerIfName, peer.IfIndex, false); err != nil {
			return err
		}
	}

	return nil
}

func (t *NetemTopologyManager) setupMgntLink(node *NetemNode) error {
	if t.mgntNet == nil {
		return nil
	}

	rootNs := link.GetRootNetns()
	defer rootNs.Close()

	peerNetns, err := node.Instance.GetNetns()
	if err != nil {
		return err
	}

	mgntIfname := fmt.Sprintf("%s%s%s.m", options.NETEM_ID, t.prjID, node.Instance.GetShortName())
	peerIfname := "mgnt"
	veth, err := link.CreateVethLink(
		mgntIfname, rootNs,
		peerIfname, peerNetns,
	)
	if err != nil {
		return fmt.Errorf(
			"unable to create mgnt link for node %s: %v",
			node.Instance.GetName(), err,
		)
	}

	// set interface up
	if err := link.SetInterfaceState(veth.Name, rootNs, link.IFSTATE_UP); err != nil {
		return err
	}
	if err := link.SetInterfaceState(veth.PeerName, peerNetns, link.IFSTATE_UP); err != nil {
		return err
	}

	if err := t.mgntNet.AttachInterface(veth.Name); err != nil {
		return err
	}
	node.Instance.AttachMgntInterface(peerIfname, peerNetns, node.Config.Mgnt.Address)

	return nil
}

func (t *NetemTopologyManager) setupLink(l *NetemLink, configure bool) error {
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
	if err := l.SetPeer1Netem(peer1IfName, peer1Netns); err != nil {
		return err
	}
	if err := l.SetPeer2Netem(peer2IfName, peer2Netns); err != nil {
		return err
	}

	// create tbf qdisc if necessary
	if err := l.SetPeer1TBF(peer1IfName, peer1Netns); err != nil {
		return err
	}
	if err := l.SetPeer2TBF(peer2IfName, peer2Netns); err != nil {
		return err
	}

	// set interface up
	if err := link.SetInterfaceState(peer1IfName, peer1Netns, link.IFSTATE_UP); err != nil {
		return err
	}
	if err := link.SetInterfaceState(peer2IfName, peer2Netns, link.IFSTATE_UP); err != nil {
		return err
	}

	// attach interfaces to nodes
	if err := l.Peer1.Node.AttachInterface(peer1IfName, l.Peer1.IfIndex, configure); err != nil {
		return err
	}
	if err := l.Peer2.Node.AttachInterface(peer2IfName, l.Peer2.IfIndex, configure); err != nil {
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
		HasPeer1Netem: false,
		HasPeer2Netem: false,
		HasPeer1Tbf:   false,
		HasPeer2Tbf:   false,
		Config:        linkCfg,
	}
	if err := t.setupLink(link, true); err != nil {
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

	peer2Netns, err := l.Peer2.Node.GetNetns()
	if err != nil {
		return err
	}

	// update config
	l.Config.Peer1QoS = linkCfg.Peer1QoS
	l.Config.Peer2QoS = linkCfg.Peer2QoS

	peer1IfName := l.Peer1.Node.GetInterfaceName(l.Peer1.IfIndex)
	peer2IfName := l.Peer2.Node.GetInterfaceName(l.Peer2.IfIndex)

	// create/update netem qdisc if necessary
	if err := l.SetPeer1Netem(peer1IfName, peer1Netns); err != nil {
		return err
	}
	if err := l.SetPeer2Netem(peer2IfName, peer2Netns); err != nil {
		return err
	}

	// create tbf qdisc if necessary
	if err := l.SetPeer1TBF(peer1IfName, peer1Netns); err != nil {
		return err
	}
	if err := l.SetPeer2TBF(peer2IfName, peer2Netns); err != nil {
		return err
	}

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

	// close all bridges
	rootNs := link.GetRootNetns()
	defer rootNs.Close()
	for _, br := range t.bridges {
		for _, peer := range br.Peers {
			ifName := fmt.Sprintf(
				"%s%s%s.%d", options.NETEM_ID, t.prjID,
				peer.Node.GetShortName(), peer.IfIndex)
			if err := link.DeleteLink(ifName, rootNs); err != nil {
				t.logger.Warnf("Error when deleting link %s: %v", ifName, err)
			}

		}

		if err := link.DeleteLink(br.Name, rootNs); err != nil {
			t.logger.Warnf("Error when deleting bridge %s: %v", br.Name, err)
		}

		if progressCh != nil {
			progressCh <- TopologyRunCloseProgressT{Code: CLOSE_BRIDGE}
		}
	}

	t.nodes = make([]NetemNode, 0)
	t.links = make([]*NetemLink, 0)
	t.bridges = make([]*NetemBridge, 0)
	t.IdGenerator.Close()

	// close OVS instance
	if err := ovs.CloseOvsInstance(t.prjID); err != nil {
		t.logger.Warnf("Error when closing ovswitch instance: %v", err)
	}
	t.ovsInstance = nil

	// close mgnt network
	if t.mgntNet != nil {
		if err := t.mgntNet.Close(); err != nil {
			t.logger.Warnf("Error when closing mgnt network: %v", err)
		}
		t.mgntNet = nil
	}

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
