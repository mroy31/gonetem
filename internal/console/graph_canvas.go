package console

import (
	"image/color"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
)

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

// addPt appends a waypoint, dropping consecutive duplicates.
func addPt(pts []graphPoint, p graphPoint) []graphPoint {
	if n := len(pts); n > 0 && pts[n-1] == p {
		return pts
	}
	return append(pts, p)
}

func centerOf(n *graphNode) int { return n.x + n.w()/2 }

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
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
func (c *graphCanvas) attachAt(n *graphNode, arm int, linkX int, col color.Color) {
	var x, y int
	switch arm {
	case armN:
		x, y = linkX, n.y // Use link's X coordinate, not center
	case armS:
		x, y = linkX, n.y+2 // Use link's X coordinate, not center
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
func (c *graphCanvas) attach(n *graphNode, arm int, col color.Color) {
	c.attachAt(n, arm, centerOf(n), col)
}

// dirBit returns the direction bit pointing from one point to the next.
func dirBit(from, to graphPoint) int {
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
func (c *graphCanvas) placeLabel(pts []graphPoint, fromEnd bool, text string, col color.Color) {
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

func makeLabelSeg(a, b graphPoint) labelSeg {
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
	// A segment walked leftward or upward would mirror the text on screen
	// ("10" reading as "01"): emit the runes in reverse so they always read
	// left-to-right / top-to-bottom.
	backward := s.dx < 0 || s.dy < 0
	for k, r := range rText {
		if backward {
			r = rText[len(rText)-1-k]
		}
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
func (c *graphCanvas) drawRoute(l *graphLink, col color.Color) {
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
		if l.labelBeside {
			c.placeLabelBeside(pts, true, l.ia, col)
			c.placeLabelBeside(pts, false, l.ib, col)
		} else {
			c.placeLabel(pts, true, l.ia, col)
			c.placeLabel(pts, false, l.ib, col)
		}
	}
}

// placeLabelBeside paints a label horizontally on the anchor end's row,
// starting next to the node frame: rightward if the space is free, then
// leftward; it falls back to along-segment placement as a last resort.
// Node content is never overwritten.
func (c *graphCanvas) placeLabelBeside(pts []graphPoint, fromEnd bool, text string, col color.Color) {
	if text == "" || len(pts) < 2 {
		return
	}
	rText := []rune(text)
	anchor := pts[0]
	if !fromEnd {
		anchor = pts[len(pts)-1]
	}
	free := func(x0, dx int) bool {
		for k := range len(rText) {
			x, y := x0+dx*k, anchor.y
			if x < 0 || y < 0 || x >= c.width || y >= c.height {
				return false
			}
			cell := &c.cells[y][x]
			if cell.s != " " && !cell.link {
				return false
			}
		}
		return true
	}
	if free(anchor.x+2, 1) {
		c.paintLabel(labelSeg{x: anchor.x, y: anchor.y, dx: 1, n: c.width}, 2, rText, col)
		return
	}
	if free(anchor.x-2, -1) {
		// paint left-to-right from the start of the left run so the text
		// never mirrors
		c.paintLabel(labelSeg{x: anchor.x - 1 - len(rText), y: anchor.y, dx: 1,
			n: len(rText) + 1}, 0, rText, col)
		return
	}
	c.placeLabel(pts, fromEnd, text, col)
}

// drawNode paints a node as a framed box, e.g.
//
//	┌────┐
//	│ R1 │
//	└────┘
//
// with the frame colored by node type and the name bolded (inverted when
// the node is selected).
func (c *graphCanvas) drawNode(n *graphNode) {
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

// panelLine is one row of a modal panel: plain text plus the style to
// render each of its characters with.
type panelLine struct {
	text  string
	style lipgloss.Style
}

// overlay stamps a modal panel of styled lines centered on (cx, cy) of the
// canvas. The rectangle it covers is blanked first so the panel reads
// cleanly over whatever the graph drew underneath.
func (c *graphCanvas) overlay(cx, cy int, lines []panelLine) {
	if len(lines) == 0 {
		return
	}
	w := 0
	for _, l := range lines {
		if n := utf8.RuneCountInString(l.text); n > w {
			w = n
		}
	}
	x0, y0 := cx-w/2, cy-len(lines)/2
	if x0+w > c.width {
		x0 = c.width - w
	}
	if x0 < 0 {
		x0 = 0
	}
	if y0+len(lines) > c.height {
		y0 = c.height - len(lines)
	}
	if y0 < 0 {
		y0 = 0
	}
	for i, l := range lines {
		y := y0 + i
		if y < 0 || y >= c.height {
			continue
		}
		runes := []rune(l.text)
		for j := 0; j < w; j++ {
			x := x0 + j
			if x < 0 || x >= c.width {
				continue
			}
			cell := &c.cells[y][x]
			*cell = gridCell{s: " "}
			if j < len(runes) {
				cell.s = l.style.Render(string(runes[j]))
			}
		}
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
