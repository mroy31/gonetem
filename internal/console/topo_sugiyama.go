package console

import (
	"math"
	"sort"
)

/*********************************
 * Sugiyama / hierarchical layout
 *********************************/

// layoutSugiyama arranges each connected component as a top-down
// hierarchy in the spirit of the Sugiyama framework: a root (the highest
// degree node) sits in layer 0 and the BFS depth gives the layer index,
// which guarantees every edge is either flat inside a layer or connects
// two adjacent layers (so no dummy nodes are needed). Barycenter sweeps
// reduce crossings inside the layers, a straightening pass pulls nodes
// towards the mean x of their neighbours, and components are placed side
// by side. Links use short orthogonal routes through the gap rows
// between layers.
func (g *topoGraph) layoutSugiyama() {
	if len(g.nodes) == 0 {
		return
	}
	maxBoxW := 0
	for _, n := range g.nodes {
		if w := n.w(); w > maxBoxW {
			maxBoxW = w
		}
	}
	cellW := maxBoxW + 2
	cellH := 8 // 3 rows of frame + 5 gap rows: 3 routing channels + label stubs

	xOff := 0
	for _, comp := range g.components() {
		xOff += g.sugiyamaComponent(comp, cellW, cellH, xOff) + 2
	}
	// Assign link indices for distributed attachment points, then pick a
	// routing row per link so overlapping horizontal spans never share a row
	g.assignLinkIndices()
	g.assignGapRows()
	for i := range g.links {
		routeHierarchical(&g.links[i])
	}
}

// sugiyamaComponent lays out one connected component with its cells
// starting at column xOff and returns its width in cells.
func (g *topoGraph) sugiyamaComponent(comp []*topoGraphNode, cellW, cellH, xOff int) int {
	// root: highest degree, deterministic tie-break on name, unless the
	// user pinned a specific node at the top with the root picker
	root := comp[0]
	for _, n := range comp[1:] {
		dn, dr := len(g.neighbors(n)), len(g.neighbors(root))
		if dn > dr || (dn == dr && n.name < root.name) {
			root = n
		}
	}
	if g.root != nil {
		for _, n := range comp {
			if n == g.root {
				root = n
				break
			}
		}
	}

	// BFS layering: depth from the root
	depth := map[*topoGraphNode]int{root: 0}
	queue := []*topoGraphNode{root}
	maxDepth := 0
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, nb := range g.neighbors(cur) {
			if _, seen := depth[nb]; !seen {
				depth[nb] = depth[cur] + 1
				if depth[nb] > maxDepth {
					maxDepth = depth[nb]
				}
				queue = append(queue, nb)
			}
		}
	}
	layers := make([][]*topoGraphNode, maxDepth+1)
	for _, n := range comp {
		layers[depth[n]] = append(layers[depth[n]], n)
	}

	// order index per node (deterministic within layers)
	pos := map[*topoGraphNode]float64{}
	reindex := func() {
		for _, layer := range layers {
			for i, n := range layer {
				pos[n] = float64(i)
			}
		}
	}
	reindex()

	// barycenter crossing reduction: alternate down/up sweeps ordering
	// each layer by the mean position of its neighbours in the reference
	// layer (ties keep the current order, then break on name)
	sortByBary := func(layer, ref []*topoGraphNode) {
		refSet := make(map[*topoGraphNode]bool, len(ref))
		for _, r := range ref {
			refSet[r] = true
		}
		key := make(map[*topoGraphNode]float64, len(layer))
		for _, n := range layer {
			sum, count := 0.0, 0
			for _, nb := range g.neighbors(n) {
				if refSet[nb] {
					sum += pos[nb]
					count++
				}
			}
			if count > 0 {
				key[n] = sum / float64(count)
			} else {
				key[n] = pos[n]
			}
		}
		sort.SliceStable(layer, func(i, j int) bool {
			if key[layer[i]] != key[layer[j]] {
				return key[layer[i]] < key[layer[j]]
			}
			return layer[i].name < layer[j].name
		})
		reindex()
	}
	for iter := 0; iter < 6; iter++ {
		for i := 1; i < len(layers); i++ {
			sortByBary(layers[i], layers[i-1])
		}
		for i := len(layers) - 2; i >= 0; i-- {
			sortByBary(layers[i], layers[i+1])
		}
	}

	// straighten x: start from the order index and pull each node to the
	// mean x of its upper (then up-pass: lower) neighbours while keeping
	// the layer order and at least one cell of horizontal spacing
	x := map[*topoGraphNode]float64{}
	for _, layer := range layers {
		for i, n := range layer {
			x[n] = float64(i)
		}
	}
	straighten := func(layer, ref []*topoGraphNode, forward bool) {
		refSet := make(map[*topoGraphNode]bool, len(ref))
		for _, r := range ref {
			refSet[r] = true
		}
		idxs := make([]int, len(layer))
		for i := range idxs {
			idxs[i] = i
		}
		if !forward {
			for i, j := 0, len(idxs)-1; i < j; i, j = i+1, j-1 {
				idxs[i], idxs[j] = idxs[j], idxs[i]
			}
		}
		for _, i := range idxs {
			n := layer[i]
			sum, count := 0.0, 0
			for _, nb := range g.neighbors(n) {
				if refSet[nb] {
					sum += x[nb]
					count++
				}
			}
			if count == 0 {
				continue
			}
			want := sum / float64(count)
			lo, hi := math.Inf(-1), math.Inf(1)
			if i > 0 {
				lo = x[layer[i-1]] + 1
			}
			if i < len(layer)-1 {
				hi = x[layer[i+1]] - 1
			}
			if lo <= hi {
				x[n] = math.Max(lo, math.Min(hi, want))
			}
		}
	}
	for iter := 0; iter < 3; iter++ {
		for i := 1; i < len(layers); i++ {
			straighten(layers[i], layers[i-1], iter%2 == 0)
		}
		for i := len(layers) - 2; i >= 0; i-- {
			straighten(layers[i], layers[i+1], iter%2 == 0)
		}
	}

	// group by subnet: reorder every layer so the nodes of a common
	// broadcast domain sit adjacent, keeping the barycenter order as the
	// secondary tie-break so layering quality is preserved.
	_, bySub := g.subnetMap()
	subSet := map[string]bool{}
	for _, ns := range bySub {
		for _, s := range ns {
			subSet[s] = true
		}
	}
	subNames := make([]string, 0, len(subSet))
	for s := range subSet {
		subNames = append(subNames, s)
	}
	sort.Strings(subNames)
	subOrder := map[string]int{}
	for i, name := range subNames {
		subOrder[name] = i
	}
	for _, layer := range layers {
		sort.SliceStable(layer, func(i, j int) bool {
			si := subOrder[firstSubnet(layer[i], bySub)]
			sj := subOrder[firstSubnet(layer[j], bySub)]
			if si != sj {
				return si < sj
			}
			return layer[i].name < layer[j].name
		})
		reindex()
	}

	// place onto the cell grid
	minX := math.Inf(1)
	for _, v := range x {
		if v < minX {
			minX = v
		}
	}
	widthCells := 0
	for _, n := range comp {
		cx := int(math.Round(x[n] - minX))
		if cx > widthCells {
			widthCells = cx
		}
		// Standard spacing: nodes placed at regular cellW intervals
		n.x = (cx+xOff)*cellW + 1
		n.y = depth[n]*cellH + 1
	}
	return widthCells
}

// assignGapRows picks the horizontal routing row for every link so that two
// links whose horizontal spans overlap never share the same row. Links are
// grouped per gap region (identified by the y of the upper frame), longest
// spans are placed first, and each link takes the first routing row
// (topY+4, +5, +6) whose already-reserved intervals leave its span free.
// When every row would overlap, the row with the smallest overlap is kept so
// the layout stays deterministic.
func (g *topoGraph) assignGapRows() {
	type gapLink struct {
		link       *topoGraphLink
		topY, loX  int
		hiX        int
	}
	groups := map[int][]*gapLink{}
	var tops []int
	for i := range g.links {
		l := &g.links[i]
		xa := attachPointX(l.a, l.indexA, l.degreeA)
		xb := attachPointX(l.b, l.indexB, l.degreeB)
		lo, hi := xa, xb
		if lo > hi {
			lo, hi = hi, lo
		}
		topY := l.a.y
		if l.b.y < topY {
			topY = l.b.y
		}
		if _, ok := groups[topY]; !ok {
			tops = append(tops, topY)
		}
		groups[topY] = append(groups[topY], &gapLink{link: l, topY: topY, loX: lo, hiX: hi})
	}
	sort.Ints(tops)
	for _, topY := range tops {
		gl := groups[topY]
		// deterministic order: widest span first, then leftmost, then names
		sort.SliceStable(gl, func(i, j int) bool {
			wi, wj := gl[i].hiX-gl[i].loX, gl[j].hiX-gl[j].loX
			if wi != wj {
				return wi > wj
			}
			if gl[i].loX != gl[j].loX {
				return gl[i].loX < gl[j].loX
			}
			if gl[i].link.a.name != gl[j].link.a.name {
				return gl[i].link.a.name < gl[j].link.a.name
			}
			return gl[i].link.b.name < gl[j].link.b.name
		})
		var rows [3][][2]int // reserved intervals per routing row
		for _, e := range gl {
			best, bestOv := 0, -1
			for r := range rows {
				ov := 0
				for _, iv := range rows[r] {
					if a, b := max(e.loX, iv[0]), min(e.hiX, iv[1]); b >= a {
						ov += b - a + 1
					}
				}
				if ov == 0 {
					best, bestOv = r, 0
					break
				}
				if bestOv < 0 || ov < bestOv {
					best, bestOv = r, ov
				}
			}
			rows[best] = append(rows[best], [2]int{e.loX, e.hiX})
			e.link.row = topY + 4 + best
		}
	}
}

// routeHierarchical gives a link its orthogonal route between the frames.
// Node frame: top border at y, content y+1, bottom border y+2; layers are
// cellH=8 rows apart, so the gap between two frames spans five rows
// (topY+3 .. topY+7). The horizontal segment runs on the row chosen by
// assignGapRows (topY+4..+6, collision free), while the outer rows stay
// free for the interface labels, painted on the stub right next to each
// node's own border. The attachment arms and points come from
// assignLinkIndices, where north and south borders are spread separately.
func routeHierarchical(l *topoGraphLink) {
	xA := attachPointX(l.a, l.indexA, l.degreeA)
	xB := attachPointX(l.b, l.indexB, l.degreeB)

	armA, armB := l.armA, l.armB
	topY := l.a.y
	if l.b.y < topY {
		topY = l.b.y
	}
	row := l.row

	var pts []topoPoint
	if armA == armS {
		pts = addPt(pts, topoPoint{xA, topY + 2}) // south border
	} else {
		pts = addPt(pts, topoPoint{xA, topY + 7}) // north border of the lower frame
	}
	if xA != xB {
		pts = addPt(pts, topoPoint{xA, row})
		pts = addPt(pts, topoPoint{xB, row})
	}
	if armB == armS {
		pts = addPt(pts, topoPoint{xB, topY + 2}) // south border
	} else {
		pts = addPt(pts, topoPoint{xB, topY + 7}) // north border of the lower frame
	}
	l.pts = pts
	l.armA, l.armB = armA, armB
}

// sideArms returns the frame sides a link attaches to on each of its
// endpoints: links between layers leave the upper frame south and enter the
// lower frame north; flat links (same layer) leave both frames south.
func sideArms(a, b *topoGraphNode) (int, int) {
	switch {
	case a.y == b.y:
		return armS, armS
	case a.y < b.y:
		return armS, armN
	default:
		return armN, armS
	}
}

// assignLinkIndices computes the attachment arms and per-side indices of
// every link endpoint, once the nodes are placed. North and south borders
// are numbered independently: a link entering the top of a node never
// competes for attachment slots with links leaving its bottom, so each
// border can spread its own links across the full frame width.
func (g *topoGraph) assignLinkIndices() {
	southDeg := map[*topoGraphNode]int{}
	northDeg := map[*topoGraphNode]int{}
	for i := range g.links {
		l := &g.links[i]
		l.armA, l.armB = sideArms(l.a, l.b)
		if l.armA == armS {
			southDeg[l.a]++
		} else {
			northDeg[l.a]++
		}
		if l.armB == armS {
			southDeg[l.b]++
		} else {
			northDeg[l.b]++
		}
	}

	southIdx := map[*topoGraphNode]int{}
	northIdx := map[*topoGraphNode]int{}
	for i := range g.links {
		l := &g.links[i]
		if l.armA == armS {
			l.indexA, l.degreeA = southIdx[l.a], southDeg[l.a]
			southIdx[l.a]++
		} else {
			l.indexA, l.degreeA = northIdx[l.a], northDeg[l.a]
			northIdx[l.a]++
		}
		if l.armB == armS {
			l.indexB, l.degreeB = southIdx[l.b], southDeg[l.b]
			southIdx[l.b]++
		} else {
			l.indexB, l.degreeB = northIdx[l.b], northDeg[l.b]
			northIdx[l.b]++
		}
	}
}

// attachPointX returns the x-coordinate for a link attachment point on a
// node's frame, distributed based on the link's index among the links
// connected to that node: a single link uses the frame centre, two links
// spread to the inner edges, and denser nodes distribute proportionally
// across the frame width.
func attachPointX(n *topoGraphNode, index, degree int) int {
	left := n.x
	right := n.x + n.w() - 1
	if degree <= 1 {
		return centerOf(n)
	}
	if degree == 2 {
		if index == 0 {
			return left + 1
		}
		return right - 1
	}
	x := left + int(float64(right-left)*float64(index)/float64(degree-1))
	if x < left {
		x = left
	}
	if x > right {
		x = right
	}
	return x
}
