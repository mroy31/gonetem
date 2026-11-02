package console

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mroy31/gonetem/internal/proto"
	"go.dalton.dog/bubbleup/v2"
)

/*********************************
 * FRR link-state database JSON
 *********************************/

// The JSON shapes come from FRR 10.x `show ip ospf database router|network
// json` (see the testdata/ospf_* fixtures). A running ospfd without an OSPF
// instance answers with an empty object, so a missing routerId means the
// node carries no usable OSPF data.

type ospfRouterDB struct {
	RouterId string `json:"routerId"`
	States   struct {
		Areas map[string][]ospfRouterLsa `json:"areas"`
	} `json:"routerLinkStates"`
}

type ospfRouterLsa struct {
	LinkStateId       string                    `json:"linkStateId"`
	AdvertisingRouter string                    `json:"advertisingRouter"`
	Links             map[string]ospfRouterLink `json:"routerLinks"`
}

type ospfRouterLink struct {
	LinkType                string `json:"linkType"`
	DesignatedRouterAddress string `json:"designatedRouterAddress"`
	RouterInterfaceAddress  string `json:"routerInterfaceAddress"`
	NeighborRouterId        string `json:"neighborRouterId"`
	Tos0Metric              int    `json:"tos0Metric"`
}

type ospfNetworkDB struct {
	RouterId string `json:"routerId"`
	States   struct {
		Areas map[string][]ospfNetworkLsa `json:"areas"`
	} `json:"networkLinkStates"`
}

type ospfNetworkLsa struct {
	LinkStateId       string `json:"linkStateId"` // DR interface address
	AdvertisingRouter string `json:"advertisingRouter"`
	NetworkMask       int    `json:"networkMask"` // prefix length
}

// ospfNoDataError reports that the queried node answered without any OSPF
// information (e.g. ospfd running with no OSPF instance configured).
type ospfNoDataError struct{}

func (e *ospfNoDataError) Error() string {
	return "node exposes no usable OSPF data"
}

// ospfRouterId extracts the querying router's own router-id from the
// router-database JSON. An answer without it carries no OSPF information.
func ospfRouterId(routerDBJson []byte) (string, error) {
	var db ospfRouterDB
	if err := json.Unmarshal(routerDBJson, &db); err != nil {
		return "", fmt.Errorf("unable to parse OSPF router database: %w", err)
	}
	if db.RouterId == "" {
		return "", &ospfNoDataError{}
	}
	return db.RouterId, nil
}

// ospfSubnet renders the subnet prefix of a transit network: the network
// LSA carries the DR's interface address and the prefix length; the subnet
// is that address masked. Unparsable input labels with the bare address.
func ospfSubnet(addr string, maskLen int) string {
	prefix, err := netip.ParsePrefix(addr + "/" + strconv.Itoa(maskLen))
	if err != nil {
		return addr
	}
	return prefix.Masked().String()
}

// buildOspfGraph turns the two FRR database JSON documents into a topoGraph:
// router vertices labeled by router-id, transit network vertices labeled by
// subnet prefix, point-to-point adjacencies as direct router edges, costs as
// router-side edge labels. All areas merge into one graph — a router-id
// appearing in several areas yields a single shared vertex — and the
// queried router is pinned as layout root.
func buildOspfGraph(routerDBJson, networkDBJson []byte) (*topoGraph, error) {
	routerId, err := ospfRouterId(routerDBJson)
	if err != nil {
		return nil, err
	}

	var routerDB ospfRouterDB
	if err := json.Unmarshal(routerDBJson, &routerDB); err != nil {
		return nil, fmt.Errorf("unable to parse OSPF router database: %w", err)
	}
	var networkDB ospfNetworkDB
	if err := json.Unmarshal(networkDBJson, &networkDB); err != nil {
		return nil, fmt.Errorf("unable to parse OSPF network database: %w", err)
	}

	g := &topoGraph{}
	byName := make(map[string]*topoGraphNode)
	vertex := func(name, typ string) *topoGraphNode {
		if n, ok := byName[name]; ok {
			return n
		}
		n := &topoGraphNode{name: name, typ: typ}
		byName[name] = n
		g.nodes = append(g.nodes, n)
		return n
	}

	// Router vertices first, so every originating router exists even when
	// all of its links are unusable.
	originated := make(map[string]bool)
	for _, area := range sortedKeys(routerDB.States.Areas) {
		for _, lsa := range routerDB.States.Areas[area] {
			if lsa.AdvertisingRouter != "" {
				vertex(lsa.AdvertisingRouter, "ospf.router")
				originated[lsa.AdvertisingRouter] = true
			}
		}
	}

	// Subnet label per transit network, keyed by the DR interface address
	// (the link-state id of its network-LSA). A missing network-LSA — a
	// one-sided adjacency — degrades to labeling with the DR address.
	transitName := make(map[string]string)
	for _, area := range sortedKeys(networkDB.States.Areas) {
		for _, lsa := range networkDB.States.Areas[area] {
			transitName[lsa.LinkStateId] = ospfSubnet(lsa.LinkStateId, lsa.NetworkMask)
		}
	}

	transitSeen := make(map[[2]string]bool)
	p2pSeen := make(map[[2]string]bool)

	for _, area := range sortedKeys(routerDB.States.Areas) {
		for _, lsa := range routerDB.States.Areas[area] {
			if lsa.AdvertisingRouter == "" {
				continue
			}
			owner := byName[lsa.AdvertisingRouter]

			for _, li := range sortedKeys(lsa.Links) {
				link := lsa.Links[li]
				cost := strconv.Itoa(link.Tos0Metric)
				switch {
				case strings.HasPrefix(link.LinkType, "a Transit"):
					dr := link.DesignatedRouterAddress
					if dr == "" {
						continue
					}
					name, ok := transitName[dr]
					if !ok {
						name = dr // degraded fallback: one-sided adjacency
					}
					net := vertex(name, "ospf.transit")
					key := [2]string{owner.name, net.name}
					if transitSeen[key] {
						continue
					}
					transitSeen[key] = true
					g.links = append(g.links, topoGraphLink{a: owner, b: net, ia: cost, ib: "", labelBeside: true})

				case strings.HasPrefix(link.LinkType, "another Router"):
					if link.NeighborRouterId == "" {
						continue
					}
					neighbor := vertex(link.NeighborRouterId, "ospf.router")
					// Both ends of a p2p adjacency list it, each with its
					// own interface address. Keep one edge per direction
					// pair: the smaller router-id's view wins, unless the
					// other end originates no router-LSA (one-sided view),
					// in which case this LSA is all the graph can show.
					if link.NeighborRouterId < owner.name && originated[link.NeighborRouterId] {
						continue
					}
					key := [2]string{owner.name, link.RouterInterfaceAddress}
					if p2pSeen[key] {
						continue
					}
					p2pSeen[key] = true
					g.links = append(g.links, topoGraphLink{
						a: owner, b: neighbor,
						ia: cost, ib: link.RouterInterfaceAddress,
						labelBeside: true,
					})
				}
				// Stub Network and other link types carry no topology edge
			}
		}
	}

	g.root = vertex(routerId, "ospf.router")
	return g, nil
}

// OspfAreaViewText renders the OSPF area view headlessly (no TUI): the
// graph structure followed by the plain-text canvas. Used by headless
// verification and documentation tooling.
func OspfAreaViewText(routerDBJson, networkDBJson []byte) (string, error) {
	g, err := buildOspfGraph(routerDBJson, networkDBJson)
	if err != nil {
		return "", err
	}
	canvas := renderGraph(g)
	out := fmt.Sprintf("root: %s\n", g.root.name)
	for _, n := range g.nodes {
		out += fmt.Sprintf("vertex: %s [%s]\n", n.name, n.typ)
	}
	for _, l := range g.links {
		out += fmt.Sprintf("link: %s <-> %s (a-label %q b-label %q)\n",
			l.a.name, l.b.name, l.ia, l.ib)
	}
	out += "--- canvas ---\n"
	for _, line := range canvas.lines() {
		out += ansi.Strip(line) + "\n"
	}
	return out, nil
}

/*********************************
 * OSPF graph viewer
 *********************************/

// ospfTopoView adapts the shared graph viewer to the OSPF area view: the
// root stays pinned to the queried router (r is inert), and the header
// names vertices by their protocol role with the link costs.
func ospfTopoView() topoViewConfig {
	return topoViewConfig{
		rootPicker: false,
		status:     "↑↓←→ scroll   tab/n/p next vertex   q quit",
		detail: func(n *topoGraphNode, g *topoGraph) string {
			role := "Router"
			if n.typ == "ospf.transit" {
				role = "Transit network"
			}
			var parts []string
			for _, l := range g.links {
				switch {
				case l.a == n:
					parts = append(parts, "↔ "+l.b.name+" (cost "+l.ia+")")
				case l.b == n:
					parts = append(parts, "↔ "+l.a.name+" (cost "+l.ia+")")
				}
			}
			detail := role + " " + n.name
			if len(parts) > 0 {
				detail += "  ·  " + strings.Join(parts, "   ")
			}
			return detail
		},
	}
}

// newOspfTopoModel builds the area view model from the two database JSON
// documents of a router.
func newOspfTopoModel(routerDBJson, networkDBJson []byte) (*topoModel, error) {
	g, err := buildOspfGraph(routerDBJson, networkDBJson)
	if err != nil {
		return nil, err
	}
	return newPlacedTopoModel(g, ospfTopoView()), nil
}

// ospfExec is the console command implementation: the target must exist in
// the project topology and be an OSPF-capable router node, then the area
// view opens from the node's own link-state databases.
func (p *NetemPrompt) ospfExec(client proto.NetemClient, cmdArgs []string) error {
	nodeName := cmdArgs[0]

	response, err := client.ReadNetworkFile(p.getCancelContext(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		return err
	}
	g, err := parseTopology(response.GetData())
	if err != nil {
		return err
	}
	var typ string
	found := false
	for _, n := range g.nodes {
		if n.name == nodeName {
			typ, found = n.typ, true
			break
		}
	}
	if !found {
		return fmt.Errorf("Node %s: not found in the topology", nodeName)
	}
	if !strings.HasPrefix(typ, "docker.router") {
		return fmt.Errorf("Node %s: type %s does not run OSPF", nodeName, typ)
	}

	return ospfTopoExec(p, client, nodeName)
}

// ospfTopoExec fetches the router and network link-state databases from the
// node with one-shot execs and opens the area view.
func ospfTopoExec(p *NetemPrompt, client proto.NetemClient, nodeName string) error {
	ctx := p.getCancelContext()
	fetch := func(db string) ([]byte, error) {
		res, err := client.NodeExecOnce(ctx, &proto.NodeExecOnceRequest{
			PrjId: p.prjID, Node: nodeName,
			Cmd: []string{"vtysh", "-c", "show ip ospf database " + db + " json"},
		})
		if err != nil {
			return nil, err
		}
		if code := res.GetExitCode(); code != 0 {
			out := strings.TrimSpace(string(res.GetStdout()) + "\n" + string(res.GetStderr()))
			return nil, fmt.Errorf("command on %s failed with exit status %d: %s",
				nodeName, code, out)
		}
		return res.GetStdout(), nil
	}
	routerJson, err := fetch("router")
	if err != nil {
		return err
	}
	networkJson, err := fetch("network")
	if err != nil {
		return err
	}
	return runOspfView(p, routerJson, networkJson)
}

// runOspfView builds and runs the OSPF area view, handing over the terminal
// from the console TUI the same way the topology view does.
func runOspfView(p *NetemPrompt, routerJson, networkJson []byte) error {
	model, err := newOspfTopoModel(routerJson, networkJson)
	if err != nil {
		return err
	}
	if p.program == nil {
		return runTopoModel(model)
	}
	program := p.program
	p.pendingCmd = func() tea.Msg {
		if err := program.ReleaseTerminal(); err != nil {
			return promptAlertMsg{alertType: bubbleup.ErrorKey, alertMsg: fmt.Sprintf("Unable to release terminal: %v", err)}
		}
		viewErr := runTopoModel(model)
		if err := program.RestoreTerminal(); err != nil {
			return promptAlertMsg{alertType: bubbleup.ErrorKey, alertMsg: fmt.Sprintf("Unable to restore terminal: %v", err)}
		}
		if viewErr != nil {
			return promptAlertMsg{alertType: bubbleup.ErrorKey, alertMsg: fmt.Sprintf("Unable to run OSPF view: %v", viewErr)}
		}
		return nil
	}
	return nil
}
