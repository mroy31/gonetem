package console

import (
	"fmt"
	"image/color"
	"sort"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mroy31/gonetem/internal/proto"
	"go.dalton.dog/bubbleup/v2"
	"gopkg.in/yaml.v3"
)

/*********************************
 * data model
 *********************************/

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

type topoGraphNode struct {
	name string
	typ  string

	x, y     int // grid position of the node cell
	selected bool
	running  bool
}

type topoGraphLink struct {
	a, b   *topoGraphNode
	ia, ib string // interface names, e.g. "R1.2" -> "2"

	// route computed by the layout: waypoints of the orthogonal path,
	// starting next to a's frame and ending next to b's frame, plus the
	// frame sides (arm bitmask) where the path attaches.
	pts        []topoPoint
	armA, armB int
	row        int // horizontal routing row, picked to avoid overlapping spans

	// link indices for distributed attachment: which link number this is
	// among all links connecting to each node (used for spreading across node edges)
	indexA, indexB   int // link index for nodes a and b
	degreeA, degreeB int // total degree (number of links) for nodes a and b
}

// Arm bitmask directions, shared with the junction merging logic.
const (
	armN = 1
	armE = 2
	armS = 4
	armW = 8
)

// opp returns the opposite direction bit.
func opp(arm int) int {
	switch arm {
	case armN:
		return armS
	case armS:
		return armN
	case armE:
		return armW
	case armW:
		return armE
	}
	return 0
}

type topoPoint struct{ x, y int }

// addPt appends a waypoint, dropping consecutive duplicates.
func addPt(pts []topoPoint, p topoPoint) []topoPoint {
	if n := len(pts); n > 0 && pts[n-1] == p {
		return pts
	}
	return append(pts, p)
}

func centerOf(n *topoGraphNode) int { return n.x + n.w()/2 }

func isHostType(typ string) bool { return strings.HasPrefix(typ, "docker.host") }

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// w is the display width of the node's frame box.
func (n *topoGraphNode) w() int { return len(n.name) + 4 }

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

func parseTopology(data []byte) (*topoGraph, error) {
	var raw topoYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	g := &topoGraph{}
	byName := make(map[string]*topoGraphNode, len(raw.Nodes))
	for _, name := range sortedKeys(raw.Nodes) {
		n := &topoGraphNode{
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
		g.links = append(g.links, topoGraphLink{
			a: na, b: nb,
			ia: splitIface(l.Peer1), ib: splitIface(l.Peer2),
		})
	}
	return g, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

/*********************************
 * layout
 *********************************/

// topoGraph is the resolved graph ready to be laid out and rendered.
type topoGraph struct {
	nodes []*topoGraphNode
	links []topoGraphLink
}

func (g *topoGraph) neighbors(n *topoGraphNode) []*topoGraphNode {
	var out []*topoGraphNode
	for i := range g.links {
		l := &g.links[i]
		switch {
		case l.a == n:
			out = append(out, l.b)
		case l.b == n:
			out = append(out, l.a)
		}
	}
	return out
}

// normalizeCanvas shifts every node frame and routed waypoint into the
// positive coordinate space the canvas expects (min x/y becomes one).
func (g *topoGraph) normalizeCanvas() {
	minX, minY := 0, 0
	for _, n := range g.nodes {
		if n.x < minX {
			minX = n.x
		}
		if n.y < minY {
			minY = n.y
		}
	}
	for i := range g.links {
		for _, p := range g.links[i].pts {
			if p.x < minX {
				minX = p.x
			}
			if p.y < minY {
				minY = p.y
			}
		}
	}
	offX, offY := 1-minX, 1-minY
	if offX == 0 && offY == 0 {
		return
	}
	for _, n := range g.nodes {
		n.x += offX
		n.y += offY
	}
	for i := range g.links {
		for j := range g.links[i].pts {
			g.links[i].pts[j].x += offX
			g.links[i].pts[j].y += offY
		}
	}
}

func (g *topoGraph) degree(n *topoGraphNode) int {
	d := 0
	for _, l := range g.links {
		if l.a == n || l.b == n {
			d++
		}
	}
	return d
}

// components returns the connected components of the graph, one per distinct
// subgraph reachable via breadth-first traversal from an unvisited starting node.
func (g *topoGraph) components() [][]*topoGraphNode {
	seen := map[*topoGraphNode]bool{}
	var comps [][]*topoGraphNode
	for _, n := range g.nodes {
		if seen[n] {
			continue
		}
		seen[n] = true
		queue := []*topoGraphNode{n}
		var comp []*topoGraphNode
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			comp = append(comp, cur)
			for _, nb := range g.neighbors(cur) {
				if !seen[nb] {
					seen[nb] = true
					queue = append(queue, nb)
				}
			}
		}
		comps = append(comps, comp)
	}
	return comps
}

/*********************************
 * Box-drawing rendering
 *********************************/

// gridCell holds the final text of a single grid cell, already styled with
// its ANSI colors so character background is preserved. link marks cells
// painted by links so node content is never overwritten by routing.
type gridCell struct {
	s     string
	link  bool
	label bool
}

type graphCanvas struct {
	cells  [][]gridCell
	width  int
	height int
}

func newGraphCanvas(width, height int) *graphCanvas {
	cells := make([][]gridCell, height)
	for y := 0; y < height; y++ {
		cells[y] = make([]gridCell, width)
		for x := 0; x < width; x++ {
			cells[y][x] = gridCell{s: " "}
		}
	}
	return &graphCanvas{cells: cells, width: width, height: height}
}

func (c *graphCanvas) set(x, y int, ch rune, col color.Color) {
	if x < 0 || y < 0 || x >= c.width || y >= c.height {
		return
	}
	cur := c.cells[y][x].s
	if cur == " " {
		c.cells[y][x].s = lipgloss.NewStyle().Foreground(col).Render(string(ch))
		return
	}
	plain := firstRune(cur)
	joined := joinRunes(plain, ch)
	c.cells[y][x].s = lipgloss.NewStyle().Foreground(col).Render(string(joined))
}

// firstRune returns the first printable rune of a cell, skipping any ANSI
// escape sequences. CSI sequences end on a byte in the 0x40-0x7E range,
// which includes the lowercase SGR final byte "m" that lipgloss emits.
func firstRune(cell string) rune {
	s := cell
	for s != "" {
		if s[0] == 0x1b {
			if len(s) < 2 {
				return ' '
			}
			if s[1] == '[' {
				i := 2
				for i < len(s) && (s[i] < 0x40 || s[i] > 0x7e) {
					i++
				}
				if i < len(s) {
					s = s[i+1:]
					continue
				}
				return ' '
			}
			s = s[2:]
			continue
		}
		r, _ := utf8.DecodeRuneInString(s)
		return r
	}
	return ' '
}

// runeArms returns the directions (bitmask) a line glyph connects to.
func runeArms(r rune) int {
	const n, e, s, w = 1, 2, 4, 8
	switch r {
	case '─':
		return e | w
	case '│':
		return n | s
	case '┌':
		return e | s
	case '┐':
		return s | w
	case '└':
		return e | n
	case '┘':
		return n | w
	case '├':
		return n | s | e
	case '┤':
		return n | s | w
	case '┬':
		return e | w | s
	case '┴':
		return e | w | n
	case '┼':
		return n | e | s | w
	}
	return 0
}

// armsRune is the inverse mapping of runeArms.
func armsRune(arms int) rune {
	switch arms {
	case 3: // n|e
		return '└'
	case 5: // n|s
		return '│'
	case 6: // e|s
		return '┌'
	case 7: // n|e|s
		return '├'
	case 9: // n|w
		return '┘'
	case 10: // e|w
		return '─'
	case 11: // n|e|w
		return '┴'
	case 12: // s|w
		return '┐'
	case 13: // n|s|w
		return '┤'
	case 14: // e|s|w
		return '┬'
	case 15: // n|e|s|w
		return '┼'
	default:
		return ' '
	}
}

// joinRunes merges two glyphs that share a cell, keeping a correct
// box-drawing junction where possible.
func joinRunes(a, b rune) rune {
	aa, ab := runeArms(a), runeArms(b)
	if aa == 0 || ab == 0 {
		if ab == 0 {
			return a
		}
		return b
	}
	j := armsRune(aa | ab)
	if j == ' ' {
		return b
	}
	return j
}

// at returns the raw content of a cell.
func (c *graphCanvas) at(x, y int) string {
	if x < 0 || y < 0 || x >= c.width || y >= c.height {
		return " "
	}
	return c.cells[y][x].s
}

// setLink draws a link glyph, but never overwrites node content: links stop
// where a node frame begins so routing never damages a node.
func (c *graphCanvas) setLink(x, y int, ch rune, col color.Color) {
	if x < 0 || y < 0 || x >= c.width || y >= c.height {
		return
	}
	cell := &c.cells[y][x]
	if cell.label {
		return
	}
	if cell.s != " " && !cell.link {
		return
	}
	cell.link = true
	c.set(x, y, ch, col)
}

// attachAt turns a frame border cell into the junction where a link arrives
// from the given arm direction, using the specified X coordinate for N/S arms.
// This ensures the junction is drawn where the link actually connects.
func (c *graphCanvas) attachAt(n *topoGraphNode, arm int, linkX int, col color.Color) {
	var x, y int
	switch arm {
	case armN:
		x, y = linkX, n.y  // Use link's X coordinate, not center
	case armS:
		x, y = linkX, n.y+2  // Use link's X coordinate, not center
	case armE:
		x, y = n.x+n.w()-1, n.y+1
	case armW:
		x, y = n.x, n.y+1
	default:
		return
	}
	// Clamp X to node boundaries
	if x < n.x {
		x = n.x
	}
	if x > n.x+n.w()-1 {
		x = n.x + n.w() - 1
	}
	if x < 0 || y < 0 || x >= c.width || y >= c.height {
		return
	}
	cell := &c.cells[y][x]
	cur := runeArms(firstRune(cell.s))
	if cur == 0 || cell.link {
		return
	}
	if ch := armsRune(cur | arm); ch != ' ' {
		cell.s = lipgloss.NewStyle().Foreground(col).Render(string(ch))
	}
}

// attach turns a frame border cell into the junction where a link arrives
// from the given arm direction (N/S on top/bottom, E/W on the sides).
// Deprecated: Use attachAt with explicit X coordinate instead.
func (c *graphCanvas) attach(n *topoGraphNode, arm int, col color.Color) {
	c.attachAt(n, arm, centerOf(n), col)
}

// dirBit returns the direction bit pointing from one point to the next.
func dirBit(from, to topoPoint) int {
	switch {
	case to.x > from.x:
		return armE
	case to.x < from.x:
		return armW
	case to.y > from.y:
		return armS
	case to.y < from.y:
		return armN
	}
	return 0
}

// cornerOrStraight maps a direction bitmask to the glyph connecting those
// directions, falling back to a plain segment for single-direction ends.
func cornerOrStraight(arms int) rune {
	if ch := armsRune(arms); ch != ' ' {
		return ch
	}
	switch arms {
	case armE, armW, armE | armW:
		return '─'
	case armN, armS, armN | armS:
		return '│'
	}
	return 0
}

// placeLabel paints the interface name of a link endpoint, walking the
// polyline outward from the requested end. It first looks for a run of
// plain link segment cells, then for any run not already carrying a
// label, and only as a last resort paints over other content: every
// interface number stays visible because setLink refuses to overwrite
// label cells afterwards.
func (c *graphCanvas) placeLabel(pts []topoPoint, fromEnd bool, text string, col color.Color) {
	if text == "" || len(pts) < 2 {
		return
	}
	rText := []rune(text)

	var segs []labelSeg
	if fromEnd {
		segs = make([]labelSeg, 0, len(pts)-1)
		for i := 0; i+1 < len(pts); i++ {
			segs = append(segs, makeLabelSeg(pts[i], pts[i+1]))
		}
	} else {
		segs = make([]labelSeg, 0, len(pts)-1)
		for i := len(pts) - 1; i > 0; i-- {
			segs = append(segs, makeLabelSeg(pts[i], pts[i-1]))
		}
	}

	for pass := 0; pass < 2; pass++ {
		for _, s := range segs {
			for start := 1; start+len(rText)-1 <= s.n; start++ {
				fits := true
				if start > 1 {
					// keep one blank cell between distinct labels so they
					// never read as a single interface number
					px, py := s.x+s.dx*(start-1), s.y+s.dy*(start-1)
					if px >= 0 && py >= 0 && px < c.width && py < c.height && c.cells[py][px].label {
						fits = false
					}
				}
				for k := 0; fits && k < len(rText); k++ {
					cx, cy := s.x+s.dx*(start+k), s.y+s.dy*(start+k)
					if cx < 0 || cy < 0 || cx >= c.width || cy >= c.height {
						fits = false
						break
					}
					cell := &c.cells[cy][cx]
					if !cell.link || cell.label {
						fits = false
						break
					}
					if ch := firstRune(cell.s); pass == 0 && ch != '─' && ch != '│' {
						fits = false
						break
					}
				}
				if fits {
					c.paintLabel(s, start, rText, col)
					return
				}
			}
		}
	}
	if len(segs) > 0 {
		c.paintLabel(segs[0], 1, rText, col)
	}
}

// labelSeg is a straight polyline segment: start cell, unit step and the
// number of strictly inner cells (waypoint junctions excluded).
type labelSeg struct{ x, y, dx, dy, n int }

func makeLabelSeg(a, b topoPoint) labelSeg {
	s := labelSeg{x: a.x, y: a.y}
	switch {
	case b.x != a.x:
		s.dx = 1
		if b.x < a.x {
			s.dx = -1
		}
		s.n = abs(b.x-a.x) - 1
	default:
		s.dy = 1
		if b.y < a.y {
			s.dy = -1
		}
		s.n = abs(b.y-a.y) - 1
	}
	return s
}

func (c *graphCanvas) paintLabel(s labelSeg, start int, rText []rune, col color.Color) {
	style := lipgloss.NewStyle().Foreground(col).Italic(true)
	for k, r := range rText {
		cx, cy := s.x+s.dx*(start+k), s.y+s.dy*(start+k)
		if cx < 0 || cy < 0 || cx >= c.width || cy >= c.height {
			return
		}
		cell := &c.cells[cy][cx]
		cell.s = style.Render(string(r))
		cell.label = true
		cell.link = true
	}
}

// drawRoute renders a routed link: frame junctions at both ends, the
// orthogonal polyline of its waypoints, and the interface labels along the
// first segment at each end.
func (c *graphCanvas) drawRoute(l *topoGraphLink, col color.Color) {
	pts := l.pts
	if len(pts) == 0 {
		return
	}
	// Attach at the actual routing start point, not the node center
	if len(pts) > 0 {
		c.attachAt(l.a, l.armA, pts[0].x, nodeColor(l.a.typ))
		c.attachAt(l.b, l.armB, pts[len(pts)-1].x, nodeColor(l.b.typ))
	}

	for i, p := range pts {
		arms := 0
		if i > 0 {
			arms |= dirBit(p, pts[i-1])
		} else {
			arms |= opp(l.armA)
		}
		if i < len(pts)-1 {
			arms |= dirBit(p, pts[i+1])
		} else {
			arms |= opp(l.armB)
		}
		if ch := cornerOrStraight(arms); ch != 0 {
			c.setLink(p.x, p.y, ch, col)
		}
	}
	for i := 0; i+1 < len(pts); i++ {
		a, b := pts[i], pts[i+1]
		if a == b {
			continue
		}
		if a.y == b.y {
			step := 1
			if b.x < a.x {
				step = -1
			}
			for x := a.x + step; x != b.x; x += step {
				c.setLink(x, a.y, '─', col)
			}
		} else if a.x == b.x {
			step := 1
			if b.y < a.y {
				step = -1
			}
			for y := a.y + step; y != b.y; y += step {
				c.setLink(a.x, y, '│', col)
			}
		}
	}

	if len(pts) > 1 {
		c.placeLabel(pts, true, l.ia, col)
		c.placeLabel(pts, false, l.ib, col)
	}
}

// drawNode paints a node as a framed box, e.g.
//
//	┌────┐
//	│ R1 │
//	└────┘
//
// with the frame colored by node type and the name bolded (inverted when
// the node is selected).
func (c *graphCanvas) drawNode(n *topoGraphNode) {
	col := nodeColor(n.typ)
	inner := len(n.name) + 2
	top := "┌" + strings.Repeat("─", inner) + "┐"
	mid := "│ " + n.name + " │"
	bot := "└" + strings.Repeat("─", inner) + "┘"
	if n.selected {
		st := lipgloss.NewStyle().Background(col).Foreground(lipgloss.Color("0")).Bold(true)
		c.stamp(n.x, n.y, top, st)
		c.stamp(n.x, n.y+1, mid, st)
		c.stamp(n.x, n.y+2, bot, st)
		return
	}
	bd := lipgloss.NewStyle().Foreground(col)
	c.stamp(n.x, n.y, top, bd)
	c.stamp(n.x, n.y+1, mid, lipgloss.NewStyle().Bold(true).Foreground(col))
	c.stamp(n.x, n.y+2, bot, bd)
}

// renderGraph lays out the graph with Sugiyama hierarchical layout and
// renders it onto a canvas of styled cells.
func renderGraph(g *topoGraph) *graphCanvas {
	g.layoutSugiyama()

	// the layout is already normalised to positive coordinates; size the
	// canvas from the frames and the routed link waypoints
	width, height := 4, 4
	for _, n := range g.nodes {
		if e := n.x + n.w() + 2; e > width {
			width = e
		}
		if e := n.y + 5; e > height {
			height = e
		}
	}
	for _, l := range g.links {
		for _, p := range l.pts {
			if p.x+2 > width {
				width = p.x + 2
			}
			if p.y+2 > height {
				height = p.y + 2
			}
		}
	}
	canvas := newGraphCanvas(width, height)

	// draw the node frames first, then the links: links stop at the frames
	// and turn the border they attach into a junction; links attached to
	// the selected node are highlighted
	for _, n := range g.nodes {
		canvas.drawNode(n)
	}

	for i := range g.links {
		l := &g.links[i]
		col := lipgloss.Color("8")
		if l.a.selected || l.b.selected {
			// magenta: distinct from the node type colors (blue, green,
			// yellow for switches, cyan) and from the grey idle links
			col = lipgloss.Color("13")
		}
		canvas.drawRoute(l, col)
	}

	return canvas
}

// endpoint formats a "node.iface" reference for display.
func endpoint(node, iface string) string {
	if iface == "" {
		return node
	}
	return node + "." + iface
}

// linksOf returns a description of every link touching the node, e.g.
// "R1.0↔PC1.0  R1.1↔R2.1".
func (g *topoGraph) linksOf(n *topoGraphNode) string {
	var parts []string
	for _, l := range g.links {
		switch {
		case l.a == n:
			parts = append(parts, endpoint(n.name, l.ia)+"↔"+endpoint(l.b.name, l.ib))
		case l.b == n:
			parts = append(parts, endpoint(n.name, l.ib)+"↔"+endpoint(l.a.name, l.ia))
		}
	}
	return strings.Join(parts, "  ")
}

// legend renders one colored chip per node type present in the graph.
func (g *topoGraph) legend() string {
	seen := make(map[string]bool)
	var chips []string
	for _, n := range g.nodes {
		if seen[n.typ] {
			continue
		}
		seen[n.typ] = true
		label := n.typ
		if label == "" {
			label = "node"
		}
		if i := strings.LastIndex(label, "."); i >= 0 && i < len(label)-1 {
			label = label[i+1:]
		}
		chips = append(chips, lipgloss.NewStyle().Bold(true).Foreground(nodeColor(n.typ)).Render("■ "+label))
	}
	return strings.Join(chips, "  ")
}

// nodeColor picks a color by node type.
func nodeColor(typ string) color.Color {
	switch {
	case strings.HasPrefix(typ, "docker.router"):
		return lipgloss.Color("4") // blue
	case strings.HasPrefix(typ, "docker.host"):
		return lipgloss.Color("2") // green
	case typ == "ovs" || strings.HasPrefix(typ, "ovs"):
		return lipgloss.Color("3") // yellow
	default:
		return lipgloss.Color("6") // cyan
	}
}

// stamp writes a labeled text starting at the node cell, styling each
// character individually so its color and background are preserved across
// the cell grid.
func (c *graphCanvas) stamp(x, y int, text string, style lipgloss.Style) {
	if y < 0 || y >= c.height {
		return
	}
	runes := []rune(text)
	for i, r := range runes {
		cx := x + i
		if cx < 0 || cx >= c.width {
			continue
		}
		c.cells[y][cx].s = style.Render(string(r))
	}
}

func (c *graphCanvas) lines() []string {
	lines := make([]string, c.height)
	for y, row := range c.cells {
		var sb strings.Builder
		for _, cell := range row {
			sb.WriteString(cell.s)
		}
		lines[y] = sb.String()
	}
	return lines
}

// window returns the rendered lines for a rectangular viewport. Each cell is
// a single display column, so slicing cells is ANSI-safe. Columns and rows
// in a cell that fall off the canvas are rendered as blanks.
func (c *graphCanvas) window(offsetX, offsetY, viewW, viewH int) []string {
	if viewH <= 0 {
		return nil
	}
	if viewW < 0 {
		viewW = 0
	}
	lines := make([]string, viewH)
	for ly := 0; ly < viewH; ly++ {
		cy := offsetY + ly
		var sb strings.Builder
		if cy >= 0 && cy < c.height {
			row := c.cells[cy]
			for lx := 0; lx < viewW; lx++ {
				cx := offsetX + lx
				if cx >= 0 && cx < len(row) {
					sb.WriteString(row[cx].s)
				} else {
					sb.WriteByte(' ')
				}
			}
		} else {
			sb.WriteString(strings.Repeat(" ", viewW))
		}
		lines[ly] = sb.String()
	}
	return lines
}

/*********************************
 * Bubble Tea model
 *********************************/

// topoModel presents the network topology as an interactive graph.
type topoModel struct {
	graph   *topoGraph
	canvas  *graphCanvas
	width   int
	height  int
	offsetX int
	offsetY int
	sel     int // index into graph.nodes
}

func newTopoModel(data []byte) (*topoModel, error) {
	g, err := parseTopology(data)
	if err != nil {
		return nil, err
	}
	m := &topoModel{graph: g}
	if len(g.nodes) > 0 {
		g.nodes[0].selected = true
	}
	m.canvas = renderGraph(g)
	return m, nil
}

func (m *topoModel) Init() tea.Cmd { return nil }

func (m *topoModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "left":
			m.scrollX(-1)
		case "right":
			m.scrollX(1)
		case "up":
			m.scrollY(-1)
		case "down":
			m.scrollY(1)
		case "tab", "n":
			m.moveSelection(1)
			m.canvas = renderGraph(m.graph)
		case "shift+tab", "p":
			m.moveSelection(-1)
			m.canvas = renderGraph(m.graph)
		}
	}
	return m, nil
}

func (m *topoModel) scrollX(delta int) {
	m.offsetX += delta
	if m.offsetX < 0 {
		m.offsetX = 0
	}
}

func (m *topoModel) scrollY(delta int) {
	m.offsetY += delta
	if m.offsetY < 0 {
		m.offsetY = 0
	}
	if m.canvas != nil {
		maxY := m.canvas.height - m.viewRows()
		if m.offsetY > maxY {
			m.offsetY = maxY
		}
		if m.offsetY < 0 {
			m.offsetY = 0
		}
	}
}

func (m *topoModel) moveSelection(delta int) {
	if len(m.graph.nodes) == 0 {
		return
	}
	m.graph.nodes[m.sel].selected = false
	m.sel = (m.sel + delta + len(m.graph.nodes)) % len(m.graph.nodes)
	m.graph.nodes[m.sel].selected = true
}

func (m *topoModel) viewRows() int {
	rows := m.height - 4 // header + legend + status lines
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (m *topoModel) View() tea.View {
	if m.width <= 0 {
		return tea.NewView("")
	}

	node := m.graph.nodes[m.sel]
	detail := "Node " + node.name + "  [" + node.typ + "]"
	if nLinks := m.graph.linksOf(node); nLinks != "" {
		detail += "  ·  " + nLinks
	}
	if utf8.RuneCountInString(detail) > m.width {
		detail = string([]rune(detail)[:m.width-1]) + "…"
	}
	detail = strings.Replace(detail, node.name,
		lipgloss.NewStyle().Bold(true).Foreground(nodeColor(node.typ)).Render(node.name), 1)

	status := fmt.Sprintf("↑↓←→ scroll   tab/n/p next node   q quit")

	// window the canvas cells (each cell is one display column), safe
	// against cutting ANSI escapes
	body := m.canvas.window(m.offsetX, m.offsetY, m.width, m.viewRows())

	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	lines := append([]string{detail}, body...)
	lines = append(lines, m.graph.legend(), dim.Render(status))

	// pad to the full window height so every frame repaints the whole
	// alternate screen and stale lines never linger inside the viewer
	for len(lines) < m.height {
		lines = append(lines, "")
	}

	// render in the alternate screen buffer: a clean dedicated window,
	// and quitting restores the terminal exactly as it was before,
	// leaving no trace of the graph in the terminal
	view := tea.NewView(strings.Join(lines, "\n"))
	view.AltScreen = true
	return view
}

// TopologyView builds and runs the interactive topology viewer on the
// network file bytes, returning when the user quits it. The view sets
// AltScreen on its tea.View, so the graph opens in a clean window and the
// terminal is fully restored on quit with no graph left behind.
func runTopoView(data []byte) error {
	model, err := newTopoModel(data)
	if err != nil {
		return err
	}
	p := tea.NewProgram(model)
	_, err = p.Run()
	return err
}

// topoExec is the console command implementation: it fetches the network
// file and opens the graph view. Commands run inside the console event loop,
// so the viewer is handed over as a program command: the console releases
// the terminal for the nested graph program and repaints the TUI when it
// quits, instead of both programs fighting for the screen.
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
