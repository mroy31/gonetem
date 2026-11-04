package console

import (
	"image/color"
	"sort"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

/*********************************
 * data model
 *********************************/

type graphNode struct {
	name string
	typ  string

	x, y     int // grid position of the node cell
	selected bool
	running  bool
}

type graphLink struct {
	a, b   *graphNode
	ia, ib string // interface names, e.g. "R1.2" -> "2"

	// labelBeside paints the end labels horizontally beside the frame
	// instead of along the first segment: multi-character labels (link
	// costs, interface IPs) stay readable on short segments.
	labelBeside bool

	// route computed by the layout: waypoints of the orthogonal path,
	// starting next to a's frame and ending next to b's frame, plus the
	// frame sides (arm bitmask) where the path attaches.
	pts        []graphPoint
	armA, armB int
	row        int // horizontal routing row, picked to avoid overlapping spans

	// link indices for distributed attachment: which link number this is
	// among all links connecting to each node (used for spreading across node edges)
	indexA, indexB   int // link index for nodes a and b
	degreeA, degreeB int // total degree (number of links) for nodes a and b
}

type graphPoint struct{ x, y int }

// w is the display width of the node's frame box.
func (n *graphNode) w() int { return len(n.name) + 4 }

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

// graph is the resolved graph ready to be laid out and rendered.
type graph struct {
	nodes []*graphNode
	links []graphLink

	// root optionally pins the node placed at the top of the hierarchy
	// by the layout; nil keeps the default highest-degree heuristic.
	root *graphNode
}

func (g *graph) neighbors(n *graphNode) []*graphNode {
	var out []*graphNode
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
func (g *graph) normalizeCanvas() {
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

func (g *graph) degree(n *graphNode) int {
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
func (g *graph) components() [][]*graphNode {
	seen := map[*graphNode]bool{}
	var comps [][]*graphNode
	for _, n := range g.nodes {
		if seen[n] {
			continue
		}
		seen[n] = true
		queue := []*graphNode{n}
		var comp []*graphNode
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

// renderGraph lays out the graph with Sugiyama hierarchical layout and
// renders it onto a canvas of styled cells.
func renderGraph(g *graph) *graphCanvas {
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
func (g *graph) linksOf(n *graphNode) string {
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
func (g *graph) legend() string {
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
	case strings.HasPrefix(typ, "ospf.router"):
		return lipgloss.Color("4") // blue
	case typ == "ovs" || strings.HasPrefix(typ, "ovs"):
		return lipgloss.Color("3") // yellow
	case strings.HasPrefix(typ, "ospf.transit"):
		return lipgloss.Color("3") // yellow
	case strings.HasPrefix(typ, "ospf.stub"):
		return lipgloss.Color("6") // cyan
	default:
		return lipgloss.Color("6") // cyan
	}
}

/*********************************
 * Bubble Tea model
 *********************************/

// graphViewConfig adapts the interactive graph viewer to a data source:
// the detail header line, the key-hint status line, and whether the r key
// may re-pin the layout root (the OSPF view has a pinned root by design).
type graphViewConfig struct {
	rootPicker bool
	status     string
	detail     func(n *graphNode, g *graph) string
}

// graphModel presents the network topology as an interactive graph.
type graphModel struct {
	graph   *graph
	canvas  *graphCanvas
	width   int
	height  int
	offsetX int
	offsetY int
	sel     int // index into graph.nodes

	view graphViewConfig

	// root picker modal, opened with r: choose the node placed at the
	// top of the hierarchy
	rootPickerOpen bool
	pickerIdx      int // cursor in the node list
}

// newPlacedGraphModel takes an already built graph and its view config and
// prepares the first render.
func newPlacedGraphModel(g *graph, view graphViewConfig) *graphModel {
	m := &graphModel{graph: g, view: view}
	if len(g.nodes) > 0 {
		g.nodes[0].selected = true
	}
	m.canvas = renderGraph(g)
	return m
}

func (m *graphModel) Init() tea.Cmd { return nil }

func (m *graphModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.rootPickerOpen {
			m.updateRootPicker(msg)
			return m, nil
		}
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
		case "r":
			if m.view.rootPicker {
				m.openRootPicker()
			}
		}
	}
	return m, nil
}

// openRootPicker shows the modal listing every node, with the cursor on
// the current selection or the pinned root.
func (m *graphModel) openRootPicker() {
	m.rootPickerOpen = true
	m.pickerIdx = m.sel
	if m.graph.root != nil {
		for i, n := range m.graph.nodes {
			if n == m.graph.root {
				m.pickerIdx = i
				break
			}
		}
	}
}

// updateRootPicker routes keys while the modal is open.
func (m *graphModel) updateRootPicker(msg tea.KeyPressMsg) {
	switch msg.String() {
	case "esc", "r":
		m.rootPickerOpen = false
	case "up", "k":
		if m.pickerIdx > 0 {
			m.pickerIdx--
		}
	case "down", "j":
		if m.pickerIdx < len(m.graph.nodes)-1 {
			m.pickerIdx++
		}
	case "enter":
		m.graph.root = m.graph.nodes[m.pickerIdx]
		m.rootPickerOpen = false
		m.canvas = renderGraph(m.graph)
		m.offsetX, m.offsetY = 0, 0
	}
}

// rootPickerPanel builds the modal listing every node, the highlighted
// line being the one enter will place at the top of the hierarchy.
func (m *graphModel) rootPickerPanel() []panelLine {
	nodes := m.graph.nodes
	title := "Select the node at the top"
	footer := "↑↓ move   enter select   esc cancel"

	w := utf8.RuneCountInString(title)
	if l := utf8.RuneCountInString(footer); l > w {
		w = l
	}
	entries := make([]string, len(nodes))
	for i, n := range nodes {
		if m.graph.root == n {
			entries[i] = n.name + "  · top"
		} else {
			entries[i] = n.name
		}
		if l := utf8.RuneCountInString(entries[i]) + 2; l > w {
			w = l
		}
	}

	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	bold := lipgloss.NewStyle().Bold(true)
	pad := func(text string) string {
		return "│ " + text + strings.Repeat(" ", w-utf8.RuneCountInString(text)) + " │"
	}

	lines := make([]panelLine, 0, len(nodes)+4)
	lines = append(lines,
		panelLine{"┌" + strings.Repeat("─", w+2) + "┐", dim},
		panelLine{pad(title), bold})
	for i, e := range entries {
		mark := "  "
		style := lipgloss.NewStyle()
		if i == m.pickerIdx {
			mark = "▸ "
			style = lipgloss.NewStyle().Bold(true).Foreground(nodeColor(nodes[i].typ))
		}
		lines = append(lines, panelLine{pad(mark + e), style})
	}
	lines = append(lines,
		panelLine{"└" + strings.Repeat("─", w+2) + "┘", dim},
		panelLine{footer, dim})
	return lines
}

func (m *graphModel) scrollX(delta int) {
	m.offsetX += delta
	if m.offsetX < 0 {
		m.offsetX = 0
	}
}

func (m *graphModel) scrollY(delta int) {
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

func (m *graphModel) moveSelection(delta int) {
	if len(m.graph.nodes) == 0 {
		return
	}
	m.graph.nodes[m.sel].selected = false
	m.sel = (m.sel + delta + len(m.graph.nodes)) % len(m.graph.nodes)
	m.graph.nodes[m.sel].selected = true
}

func (m *graphModel) viewRows() int {
	rows := m.height - 4 // header + legend + status lines
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (m *graphModel) View() tea.View {
	if m.width <= 0 {
		return tea.NewView("")
	}

	node := m.graph.nodes[m.sel]
	detail := m.view.detail(node, m.graph)
	if utf8.RuneCountInString(detail) > m.width {
		detail = string([]rune(detail)[:m.width-1]) + "…"
	}
	detail = strings.Replace(detail, node.name,
		lipgloss.NewStyle().Bold(true).Foreground(nodeColor(node.typ)).Render(node.name), 1)

	status := m.view.status

	// the root picker modal is stamped over the canvas, centred in the
	// visible viewport
	if m.rootPickerOpen {
		m.canvas.overlay(m.offsetX+m.width/2, m.offsetY+m.viewRows()/2, m.rootPickerPanel())
	}

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

// runGraphModel runs any configured graph viewer (topology or OSPF) as a
// nested program in the alternate screen buffer.
func runGraphModel(model *graphModel) error {
	p := tea.NewProgram(model)
	_, err := p.Run()
	return err
}
