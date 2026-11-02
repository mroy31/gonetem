package console

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/mroy31/gonetem/internal/proto"
	"go.dalton.dog/bubbleup/v2"
	"gopkg.in/yaml.v3"
)

// topoYAML mirrors the subset of the network.yml topology file the graph
// view needs: nodes with their type and links between interfaces. It is
// parsed locally so the view does not depend on the server package.
type topoYAML struct {
	Nodes map[string]struct {
		Type string `yaml:"type"`
	} `yaml:"nodes"`
	Links []struct {
		Peer1 string `yaml:"peer1"`
		Peer2 string `yaml:"peer2"`
	} `yaml:"links"`
}

func splitPeer(peer string) string {
	if i := strings.LastIndex(peer, "."); i >= 0 {
		return peer[:i]
	}
	return peer
}

// splitIface returns the interface part of a "node.iface" peer reference.
func splitIface(peer string) string {
	if i := strings.LastIndex(peer, "."); i >= 0 {
		return peer[i+1:]
	}
	return ""
}

// parseTopology decodes the network.yml bytes into placed graph nodes and
// edges.

func parseTopology(data []byte) (*graph, error) {
	var raw topoYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	g := &graph{}
	byName := make(map[string]*graphNode, len(raw.Nodes))
	for _, name := range sortedKeys(raw.Nodes) {
		n := &graphNode{
			name: name,
			typ:  raw.Nodes[name].Type,
		}
		byName[name] = n
		g.nodes = append(g.nodes, n)
	}

	for _, l := range raw.Links {
		na, okA := byName[splitPeer(l.Peer1)]
		nb, okB := byName[splitPeer(l.Peer2)]
		if !okA || !okB || na == nb {
			continue
		}
		g.links = append(g.links, graphLink{
			a: na, b: nb,
			ia: splitIface(l.Peer1), ib: splitIface(l.Peer2),
		})
	}
	return g, nil
}

func defaultTopoView() graphViewConfig {
	return graphViewConfig{
		rootPicker: true,
		status:     "↑↓←→ scroll   tab/n/p next node   r top node   q quit",
		detail: func(n *graphNode, g *graph) string {
			detail := "Node " + n.name + "  [" + n.typ + "]"
			if nLinks := g.linksOf(n); nLinks != "" {
				detail += "  ·  " + nLinks
			}
			return detail
		},
	}
}

func newTopoModel(data []byte) (*graphModel, error) {
	g, err := parseTopology(data)
	if err != nil {
		return nil, err
	}
	return newPlacedGraphModel(g, defaultTopoView()), nil
}

// TopologyView builds and runs the interactive topology viewer on the
// network file bytes, returning when the user quits it. The view sets
// AltScreen on its tea.View, so the graph opens in a clean window and the

func runTopoView(data []byte) error {
	model, err := newTopoModel(data)
	if err != nil {
		return err
	}
	return runGraphModel(model)
}

// topoExec is the console command implementation: it fetches the network
// file and opens the graph view. Commands run inside the console event loop,
// so the viewer is handed over as a program command: the console releases
// the terminal for the nested graph program and repaints the TUI when it

func (p *NetemPrompt) topoExec(client proto.NetemClient, cmdArgs []string) error {
	response, err := client.ReadNetworkFile(p.getCancelContext(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		return err
	}
	data := response.GetData()

	if p.program == nil {
		// no console TUI owns the terminal: run the view directly
		return runTopoView(data)
	}

	program := p.program
	p.pendingCmd = func() tea.Msg {
		if err := program.ReleaseTerminal(); err != nil {
			return promptAlertMsg{alertType: bubbleup.ErrorKey, alertMsg: fmt.Sprintf("Unable to release terminal: %v", err)}
		}
		viewErr := runTopoView(data)
		if err := program.RestoreTerminal(); err != nil {
			return promptAlertMsg{alertType: bubbleup.ErrorKey, alertMsg: fmt.Sprintf("Unable to restore terminal: %v", err)}
		}
		if viewErr != nil {
			return promptAlertMsg{alertType: bubbleup.ErrorKey, alertMsg: fmt.Sprintf("Unable to run topology view: %v", viewErr)}
		}
		return nil
	}
	return nil
}
