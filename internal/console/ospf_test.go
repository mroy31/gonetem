package console

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mroy31/gonetem/internal/proto"
	"google.golang.org/grpc"
)

const ospfTopoYAML = `
nodes:
  R1: {type: docker.router}
  PC1: {type: docker.host}
links: []
`

// fakeOspfClient implements the two RPCs the ospf command needs; every
// other method of the embedded interface would panic if called.
type fakeOspfClient struct {
	proto.NetemClient
	execStdout []byte
	execStderr []byte
	execExit   int32
	execErr    error
	execCmds   [][]string
}

func (f *fakeOspfClient) ReadNetworkFile(context.Context, *proto.ProjectRequest, ...grpc.CallOption) (*proto.FileResponse, error) {
	return &proto.FileResponse{Data: []byte(ospfTopoYAML)}, nil
}

func (f *fakeOspfClient) NodeExecOnce(_ context.Context, in *proto.NodeExecOnceRequest, _ ...grpc.CallOption) (*proto.ExecOnceResponse, error) {
	if f.execErr != nil {
		return nil, f.execErr
	}
	f.execCmds = append(f.execCmds, in.GetCmd())
	return &proto.ExecOnceResponse{
		Status:   &proto.Status{Code: proto.StatusCode_OK},
		Stdout:   f.execStdout,
		Stderr:   f.execStderr,
		ExitCode: f.execExit,
	}, nil
}

func testPrompt() *NetemPrompt {
	return &NetemPrompt{prjID: "abc", commands: make(map[string]*NetemCommand)}
}

var errStopped = errors.New("Node R1: not running")

// ---- 4.2 command registration and completion ----

func TestOspfCommandRegisteredAndCompleted(t *testing.T) {
	p := testPrompt()
	p.RegisterCommands()
	cmd, ok := p.commands["ospfGraph"]
	if !ok {
		t.Fatal("ospfGraph command not registered")
	}
	if cmd.Usage != "ospfGraph <node_name>" || len(cmd.Args) != 1 {
		t.Fatalf("unexpected ospfGraph command shape: %+v", cmd)
	}
	if _, legacy := p.commands["ospf"]; legacy {
		t.Fatal("legacy ospf command still registered")
	}
	found := false
	for _, c := range NewPromptCompleter(p).Complete("osp") {
		if strings.Contains(c.Text, "ospfGraph") {
			found = true
		}
	}
	if !found {
		t.Fatal("completer does not offer ospfGraph")
	}
}

// ---- 4.2 / 4.3 command error paths ----

func TestOspfExec_UnknownNode(t *testing.T) {
	err := testPrompt().ospfExec(&fakeOspfClient{}, []string{"R9"})
	if err == nil || !strings.Contains(err.Error(), "R9: not found") {
		t.Fatalf("expected unknown-node error, got %v", err)
	}
}

func TestOspfExec_NotARouter(t *testing.T) {
	err := testPrompt().ospfExec(&fakeOspfClient{}, []string{"PC1"})
	if err == nil || !strings.Contains(err.Error(), "does not run OSPF") {
		t.Fatalf("expected non-router error, got %v", err)
	}
}

func TestOspfExec_NoOspfData(t *testing.T) {
	// the captured no-OSPF answer: {} with exit status 0
	err := testPrompt().ospfExec(&fakeOspfClient{execStdout: []byte("{}")}, []string{"R1"})
	if err == nil || !strings.Contains(err.Error(), "no usable OSPF data") {
		t.Fatalf("expected no-OSPF-data error, got %v", err)
	}
}

func TestOspfExec_NonZeroExitSurfacesOutput(t *testing.T) {
	client := &fakeOspfClient{
		execExit:   1,
		execStderr: []byte("% OSPF is not enabled"),
	}
	err := testPrompt().ospfExec(client, []string{"R1"})
	if err == nil {
		t.Fatal("expected an error for a failing vtysh command")
	}
	if !strings.Contains(err.Error(), "exit status 1") || !strings.Contains(err.Error(), "OSPF is not enabled") {
		t.Fatalf("error must carry exit status and vtysh output, got %v", err)
	}
}

func TestOspfExec_StoppedNode(t *testing.T) {
	client := &fakeOspfClient{execErr: errStopped}
	err := testPrompt().ospfExec(client, []string{"R1"})
	if err == nil || !strings.Contains(err.Error(), "not running") {
		t.Fatalf("expected stopped-node error, got %v", err)
	}
}

func TestOspfExec_FetchesBothDatabases(t *testing.T) {
	client := &fakeOspfClient{execStdout: []byte("{}")}
	_ = testPrompt().ospfExec(client, []string{"R1"})
	// both databases are fetched before the answer is judged usable
	if len(client.execCmds) != 2 {
		t.Fatalf("expected two database fetches, got %d calls", len(client.execCmds))
	}
	if got := strings.Join(client.execCmds[0], " "); got != "vtysh -c show ip ospf database router json" {
		t.Fatalf("unexpected first fetch: %q", got)
	}
	if got := strings.Join(client.execCmds[1], " "); got != "vtysh -c show ip ospf database network json" {
		t.Fatalf("unexpected second fetch: %q", got)
	}
}

// ---- 3.1 renderer support for ospf vertex types ----

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

// ---- 3.1 renderer support for ospf vertex types ----

func TestOspfVertexColorsAndLegend(t *testing.T) {
	if got, want := nodeColor("ospf.router"), lipgloss.Color("4"); got != want {
		t.Errorf("ospf.router color = %v, want %v", got, want)
	}
	if got, want := nodeColor("ospf.transit"), lipgloss.Color("3"); got != want {
		t.Errorf("ospf.transit color = %v, want %v", got, want)
	}

	g := &graph{
		nodes: []*graphNode{
			{name: "1.1.1.1", typ: "ospf.router"},
			{name: "10.0.12.0/24", typ: "ospf.transit"},
		},
	}
	leg := g.legend()
	if !strings.Contains(leg, "router") || !strings.Contains(leg, "transit") {
		t.Errorf("legend missing ospf chips: %q", leg)
	}
}

// ---- 3.2 database JSON to graph ----

func TestBuildOspfGraph_AbrFixtures(t *testing.T) {
	g, err := buildOspfGraph(
		readFixture(t, "ospf_abr_lsdb_router.json"),
		readFixture(t, "ospf_abr_lsdb_network.json"))
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	wantNodes := map[string]string{
		"1.1.1.1":      "ospf.router",
		"2.2.2.2":      "ospf.router",
		"3.3.3.3":      "ospf.router",
		"4.4.4.4":      "ospf.router",
		"10.0.12.0/24": "ospf.transit",
	}
	if len(g.nodes) != len(wantNodes) {
		t.Fatalf("node count %d, want %d (%v)", len(g.nodes), len(wantNodes), nodeNames(g))
	}
	for _, n := range g.nodes {
		if want, ok := wantNodes[n.name]; !ok {
			t.Fatalf("unexpected vertex %q", n.name)
		} else if n.typ != want {
			t.Fatalf("vertex %q typ %q, want %q", n.name, n.typ, want)
		}
	}

	// three edges into the transit vertex, each labeled with cost 10
	transitEdges := 0
	for _, l := range g.links {
		if l.a.name == "10.0.12.0/24" || l.b.name == "10.0.12.0/24" {
			transitEdges++
			cost := l.ia
			if l.a.name == "10.0.12.0/24" {
				cost = l.ib
			}
			// transit edges carry the cost on the router end (a)
			if l.a.name != "10.0.12.0/24" && cost != "10" {
				t.Errorf("transit edge %s↔%s router cost label %q, want 10", l.a.name, l.b.name, l.ia)
			}
			if l.a.name != "10.0.12.0/24" && l.ib != "" {
				t.Errorf("transit edge should have an empty network-end label, got %q", l.ib)
			}
		}
	}
	if transitEdges != 3 {
		t.Errorf("transit edges = %d, want 3", transitEdges)
	}

	// exactly one p2p edge 2.2.2.2↔4.4.4.4 (both LSAs describe it once)
	p2p := 0
	for _, l := range g.links {
		pair := map[string]bool{l.a.name: true, l.b.name: true}
		if pair["2.2.2.2"] && pair["4.4.4.4"] {
			p2p++
			if l.ia != "20" {
				t.Errorf("p2p router cost label %q, want 20", l.ia)
			}
			if l.ib != "10.0.24.12" {
				t.Errorf("p2p far-end label %q, want the owner interface 10.0.24.12", l.ib)
			}
		}
	}
	if p2p != 1 {
		t.Errorf("p2p edges = %d, want exactly 1", p2p)
	}

	// 4.4.4.4 is reachable only through 2.2.2.2: the areas merged at the ABR
	if g.root == nil || g.root.name != "2.2.2.2" {
		t.Errorf("root not pinned to the queried router: %v", g.root)
	}
}

func TestBuildOspfGraph_R1ViewSingleArea(t *testing.T) {
	g, err := buildOspfGraph(
		readFixture(t, "ospf_r1_lsdb_router.json"),
		readFixture(t, "ospf_r1_lsdb_network.json"))
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	// R1 is an area-0 internal router: it sees its area's routers and the
	// transit network, but not the area-1 endpoints (summaries only).
	for _, n := range g.nodes {
		if n.name == "4.4.4.4" {
			t.Fatalf("area-1 router leaked into the R1 view")
		}
	}
	if g.root == nil || g.root.name != "1.1.1.1" {
		t.Errorf("root = %v, want the 1.1.1.1 vertex", g.root)
	}
}

func TestBuildOspfGraph_OneSidedTransitFallback(t *testing.T) {
	// Same LSDB but without any network-LSA: the transit vertex degrades to
	// the DR interface address instead of dropping the edge.
	g, err := buildOspfGraph(
		readFixture(t, "ospf_abr_lsdb_router.json"),
		[]byte(`{"routerId":"2.2.2.2","networkLinkStates":{"areas":{"0.0.0.0":[]}}}`))
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	found := false
	for _, n := range g.nodes {
		if n.name == "10.0.12.13" && n.typ == "ospf.transit" {
			found = true
		}
		if n.name == "10.0.12.0/24" {
			t.Fatalf("subnet label used although no network-LSA is present")
		}
	}
	if !found {
		t.Fatalf("degraded transit vertex (DR address) missing: %v", nodeNames(g))
	}
}

func TestBuildOspfGraph_OneSidedPointToPoint(t *testing.T) {
	// Only R4's view of the p2p link: R2 originates no router-LSA, yet the
	// edge and the lone R2 vertex must still show the half-seen adjacency.
	router := `{"routerId":"4.4.4.4","routerLinkStates":{"areas":{"0.0.0.1":[
	  {"linkStateId":"4.4.4.4","advertisingRouter":"4.4.4.4","routerLinks":{
	    "link0":{"linkType":"another Router (point-to-point)",
	             "neighborRouterId":"2.2.2.2",
	             "routerInterfaceAddress":"10.0.24.14","tos0Metric":20}}}]}}}`
	g, err := buildOspfGraph([]byte(router),
		[]byte(`{"routerId":"4.4.4.4","networkLinkStates":{"areas":{}}}`))
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(g.links) != 1 {
		t.Fatalf("one-sided p2p edge missing: %d links", len(g.links))
	}
	l := g.links[0]
	if !(l.a.name == "4.4.4.4" && l.b.name == "2.2.2.2") {
		t.Fatalf("unexpected one-sided p2p edge %s↔%s", l.a.name, l.b.name)
	}
	if l.ib != "10.0.24.14" {
		t.Errorf("one-sided p2p far-end label %q, want 10.0.24.14", l.ib)
	}
}

func TestBuildOspfGraph_Deterministic(t *testing.T) {
	rv := readFixture(t, "ospf_abr_lsdb_router.json")
	nv := readFixture(t, "ospf_abr_lsdb_network.json")
	a, err := buildOspfGraph(rv, nv)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	for i := 0; i < 5; i++ {
		b, err := buildOspfGraph(rv, nv)
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		if strings.Join(nodeNames(a), ",") != strings.Join(nodeNames(b), ",") {
			t.Fatalf("node order is not deterministic")
		}
		if linkKeys(a) != linkKeys(b) {
			t.Fatalf("link order is not deterministic")
		}
	}
}

// ---- 3.3 querying router id and root pinning ----

func TestOspfRouterId(t *testing.T) {
	id, err := ospfRouterId(readFixture(t, "ospf_abr_lsdb_router.json"))
	if err != nil {
		t.Fatalf("routerId: %v", err)
	}
	if id != "2.2.2.2" {
		t.Fatalf("routerId = %q, want 2.2.2.2", id)
	}
}

func TestOspfRouterId_NoOspfData(t *testing.T) {
	// The captured no-OSPF answer: an empty JSON object, exit status 0.
	_, err := ospfRouterId(readFixture(t, "ospf_nospf_lsdb_router.json"))
	if err == nil {
		t.Fatal("expected an error for a database without routerId")
	}
	if _, ok := err.(*ospfNoDataError); !ok {
		t.Fatalf("expected ospfNoDataError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "no usable OSPF data") {
		t.Fatalf("error should identify missing OSPF data: %v", err)
	}
}

func TestOspfRouterId_Malformed(t *testing.T) {
	if _, err := ospfRouterId([]byte(`{"routerId": `)); err == nil {
		t.Fatal("expected a parse error for malformed JSON")
	}
}

func TestOspfAreaViewText_Readable(t *testing.T) {
	out, err := OspfAreaViewText(
		readFixture(t, "ospf_abr_lsdb_router.json"),
		readFixture(t, "ospf_abr_lsdb_network.json"))
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	if !strings.Contains(out, "│ 2.2.2.2 │") {
		t.Fatalf("root box must stay intact under labels:\n%s", out)
	}
	if strings.Contains(out, "2.222.2") {
		t.Fatalf("a label overwrote the root box:\n%s", out)
	}
	if !strings.Contains(out, "10.0.24.12") {
		t.Fatalf("the p2p interface label must read horizontally:\n%s", out)
	}
}

func TestOspfView_RootPickerInert(t *testing.T) {
	model, err := newOspfModel(
		readFixture(t, "ospf_abr_lsdb_router.json"),
		readFixture(t, "ospf_abr_lsdb_network.json"))
	if err != nil {
		t.Fatalf("model: %v", err)
	}
	model.Update(tea.KeyPressMsg{Text: "r", Code: 'r'})
	if model.rootPickerOpen {
		t.Fatal("r must not open the root picker in the OSPF view")
	}
	if model.graph.root == nil || model.graph.root.name != "2.2.2.2" {
		t.Fatalf("root must stay pinned: %v", model.graph.root)
	}
}

func nodeNames(g *graph) []string {
	out := make([]string, 0, len(g.nodes))
	for _, n := range g.nodes {
		out = append(out, n.name)
	}
	return out
}

func linkKeys(g *graph) string {
	var b strings.Builder
	for _, l := range g.links {
		b.WriteString(l.a.name + "|" + l.b.name + "|" + l.ia + "|" + l.ib + ";")
	}
	return b.String()
}
