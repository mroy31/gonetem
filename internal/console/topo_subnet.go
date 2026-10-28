package console

import (
	"sort"
	"strings"
)

/*********************************
 * Subnet grouping
 *********************************/

// isSwitch reports whether a node is a switching device. A subnet is
// defined by a switch and every node attached to it.
func isSwitch(typ string) bool { return typ == "ovs" || strings.HasPrefix(typ, "ovs") }

// subnet is a group of nodes that share a broadcast domain: one switch
// plus every node directly connected to it. The name is the switch node's
// name.
type subnet struct {
	name  string
	nodes []*topoGraphNode
}

// subnets returns the distinct subnet clusters of the graph. Each switch
// yields one subnet containing the switch and every direct neighbour. A
// node attached to several switches belongs to several subnets. Nodes not
// attached to any switch form a singleton "misc" subnet so the whole graph
// is covered. Clusters are ordered deterministically by switch name.
func (g *topoGraph) subnets() []*subnet {
	// member: nodes belonging to at least one switch subnet
	member := map[*topoGraphNode]bool{}
	bySwitch := map[string]*subnet{}
	for _, n := range g.nodes {
		if !isSwitch(n.typ) {
			continue
		}
		s := &subnet{name: n.name, nodes: []*topoGraphNode{n}}
		bySwitch[n.name] = s
		for _, nb := range g.neighbors(n) {
			member[nb] = true
			s.nodes = append(s.nodes, nb)
		}
	}

	var out []*subnet
	for _, name := range sortedKeys(bySwitch) {
		out = append(out, bySwitch[name])
	}

	// singleton subnet for the nodes of no switch subnet
	var misc []*topoGraphNode
	for _, n := range g.nodes {
		if !member[n] {
			misc = append(misc, n)
		}
	}
	if len(misc) > 0 {
		sort.Slice(misc, func(i, j int) bool { return misc[i].name < misc[j].name })
		out = append(out, &subnet{name: "misc", nodes: misc})
	}
	return out
}

// subnetMap returns a deterministic ordered list of subnet names plus a
// lookup from node to the sorted names of every subnet it belongs to.
func (g *topoGraph) subnetMap() ([]string, map[*topoGraphNode][]string) {
	subs := g.subnets()
	names := make([]string, 0, len(subs))
	for _, s := range subs {
		names = append(names, s.name)
	}
	byName := make(map[*topoGraphNode][]string, len(g.nodes))
	for _, s := range subs {
		for _, n := range s.nodes {
			byName[n] = append(byName[n], s.name)
		}
	}
	for _, ns := range byName {
		sort.Strings(ns)
	}
	return names, byName
}

// shareSubnet reports whether two distinct nodes belong to a common subnet.
func shareSubnet(a, b *topoGraphNode, byName map[*topoGraphNode][]string) bool {
	for _, sa := range byName[a] {
		for _, sb := range byName[b] {
			if sa == sb {
				return true
			}
		}
	}
	return false
}

// firstSubnet returns the deterministically ordered first subnet a node
// belongs to, or "" if it belongs to none (should not happen since every
// node is covered by subnets()).
func firstSubnet(n *topoGraphNode, byName map[*topoGraphNode][]string) string {
	if ns := byName[n]; len(ns) > 0 {
		return ns[0]
	}
	return ""
}

// pullSubnets biases a force-directed placement so nodes of a common
// broadcast domain stay together: every node is nudged towards the centre
// of each subnet it belongs to by a fraction `strength` of the separation
// from that centre. Multi-homed nodes are pulled toward all their subnets
// and naturally settle in between, acting as bridges.
func (g *topoGraph) pullSubnets(pos map[*topoGraphNode][2]float64, strength float64) {
	subs := g.subnets()
	if len(subs) < 2 {
		return
	}
	delta := make(map[*topoGraphNode][2]float64, len(g.nodes))
	for _, s := range subs {
		if len(s.nodes) < 2 {
			continue
		}
		var cx, cy float64
		for _, n := range s.nodes {
			cx += pos[n][0]
			cy += pos[n][1]
		}
		cx /= float64(len(s.nodes))
		cy /= float64(len(s.nodes))
		for _, n := range s.nodes {
			d := delta[n]
			d[0] += cx - pos[n][0]
			d[1] += cy - pos[n][1]
			delta[n] = d
		}
	}
	for n, d := range delta {
		p := pos[n]
		pos[n] = [2]float64{p[0] + strength*d[0], p[1] + strength*d[1]}
	}
}
