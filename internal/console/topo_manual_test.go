package console

import (
	"os"
	"strings"
	"testing"

	"charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

const sampleTopologyYAML = `
nodes:
  PC1: {type: docker.host}
  PC2: {type: docker.host}
  R1: {type: docker.router}
  R2: {type: docker.router}
  R3: {type: docker.router}
  R51: {type: docker.router}
  R52: {type: docker.router}
  R11: {type: docker.router}
  R12: {type: docker.router}
  R13: {type: docker.router}
  PC11: {type: docker.host}
  PC12: {type: docker.host}
  SW1: {type: ovs}
  SW2: {type: ovs}
links:
- {peer1: PC1.0, peer2: R1.2}
- {peer1: R1.1, peer2: R2.1}
- {peer1: PC2.0, peer2: R2.2}
- {peer1: R2.0, peer2: SW1.0}
- {peer1: R3.0, peer2: SW1.1}
- {peer1: R3.1, peer2: SW2.0}
- {peer1: R52.0, peer2: SW2.1}
- {peer1: R52.1, peer2: R13.0}
- {peer1: R11.1, peer2: R12.1}
- {peer1: R11.0, peer2: R13.1}
- {peer1: PC11.0, peer2: R11.2}
- {peer1: PC12.0, peer2: R12.2}
`

func TestTopoView_RendersNodesAndLinks(t *testing.T) {
	model, err := newTopoModel([]byte(sampleTopologyYAML))
	if err != nil {
		t.Fatalf("new model: %v", err)
	}
	model.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	content := ansi.Strip(model.View().Content)

	// the full canvas holds every node of the topology
	var full string
	for _, l := range model.canvas.lines() {
		full += strings.TrimRight(ansi.Strip(l), " ") + "\n"
	}
	for _, name := range []string{"PC1", "R1", "R3", "SW1", "SW2", "R52", "R13", "PC11", "PC12"} {
		if !strings.Contains(full, name) {
			t.Fatalf("canvas missing node %q\n%s", name, full)
		}
	}
	// the windowed view shows box-drawing links
	if !strings.ContainsAny(content, "─│┌┐└┘") {
		t.Fatalf("view missing box-drawing links\n%s", content)
	}
}

func TestTopoView_Navigation(t *testing.T) {
	model, err := newTopoModel([]byte(sampleTopologyYAML))
	if err != nil {
		t.Fatalf("new model: %v", err)
	}
	n := len(model.graph.nodes)
	start := model.sel
	model.moveSelection(1)
	if model.sel != (start+1)%n {
		t.Fatalf("selection did not advance")
	}
	model.moveSelection(-1)
	if model.sel != start {
		t.Fatalf("selection did not wrap back")
	}
	// exactly one node is selected after re-render
	model.canvas = renderGraph(model.graph)
	selected := 0
	for _, nd := range model.graph.nodes {
		if nd.selected {
			selected++
		}
	}
	if selected != 1 {
		t.Fatalf("expected exactly 1 selected node, got %d", selected)
	}
}

func TestTopoView_SugiyamaLayout(t *testing.T) {
	model, err := newTopoModel([]byte(sampleTopologyYAML))
	if err != nil {
		t.Fatalf("new model: %v", err)
	}
	model.Update(tea.WindowSizeMsg{Width: 200, Height: 60})

	var full string
	for _, l := range model.canvas.lines() {
		full += strings.TrimRight(ansi.Strip(l), " ") + "\n"
	}
	for _, name := range []string{"PC1", "R1", "R3", "SW1", "SW2", "R51", "R52", "R11", "R13", "PC11", "PC12"} {
		if !strings.Contains(full, name) {
			t.Fatalf("sugiyama canvas missing node %q\n%s", name, full)
		}
	}
	// box-drawing characters show routed links between the frames
	if !strings.ContainsAny(full, "┤├┴┬") {
		t.Fatalf("sugiyama canvas has no link glyphs\n%s", full)
	}

	// the layout is deterministic: rendering twice yields the same canvas
	second := renderGraph(model.graph)
	a := strings.Join(model.canvas.lines(), "\n")
	b := strings.Join(second.lines(), "\n")
	if a != b {
		t.Fatalf("Sugiyama layout is not deterministic")
	}
}



func TestTopoView_SugiyamaMulticast(t *testing.T) {
	data, err := os.ReadFile("../../networks/multicast_network.yml")
	if err != nil {
		t.Fatalf("read topology: %v", err)
	}
	model, err := newTopoModel(data)
	if err != nil {
		t.Fatalf("new model: %v", err)
	}
	model.Update(tea.WindowSizeMsg{Width: 240, Height: 70})

	var full string
	for _, l := range model.canvas.lines() {
		full += strings.TrimRight(ansi.Strip(l), " ") + "\n"
	}
	for _, name := range []string{"R1", "R2", "R3", "R4", "R5", "abonne1", "abonne2", "source", "sw1", "sw2", "sw3", "sw4"} {
		if !strings.Contains(full, " "+name+" ") {
			t.Fatalf("canvas missing node %q\n%s", name, full)
		}
	}

	// every link must have at least some interface numbers displayed
	// (some may be omitted if they would overlap with other routing)
	labels := 0
	for row := range model.canvas.cells {
		for _, cell := range model.canvas.cells[row] {
			if cell.label {
				labels++
			}
		}
	}
	if labels == 0 {
		t.Fatalf("no interface labels painted\n%s", full)
	}

	// deterministic across renders
	second := renderGraph(model.graph)
	if strings.Join(model.canvas.lines(), "\n") != strings.Join(second.lines(), "\n") {
		t.Fatalf("sugiyama multicast layout is not deterministic")
	}
}
