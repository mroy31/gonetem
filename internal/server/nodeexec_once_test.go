package server

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/moby/term"
	"github.com/mroy31/gonetem/internal/link"
	"github.com/mroy31/gonetem/internal/proto"
	"github.com/vishvananda/netns"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type fakeExecNode struct {
	name     string
	running  bool
	stdout   []byte
	stderr   []byte
	exitCode int
	execErr  error
	gotCmd   []string
}

func (f *fakeExecNode) GetName() string      { return f.name }
func (f *fakeExecNode) GetShortName() string { return f.name }
func (f *fakeExecNode) GetType() string      { return "docker.router" }
func (f *fakeExecNode) GetFullType() string  { return "docker.router" }
func (f *fakeExecNode) IsRunning() bool      { return f.running }
func (f *fakeExecNode) Start() error         { return nil }
func (f *fakeExecNode) Stop() error          { return nil }
func (f *fakeExecNode) GetNetns() (netns.NsHandle, error) {
	return netns.NsHandle(0), nil
}
func (f *fakeExecNode) GetInterfaceName(ifIndex int) string { return "eth0" }
func (f *fakeExecNode) AttachMgntInterface(ifName string, ns netns.NsHandle, IPAddress string) error {
	return nil
}
func (f *fakeExecNode) AttachInterface(ifName string, ifIndex int, configure bool) error {
	return nil
}
func (f *fakeExecNode) ConfigureInterfaces() error { return nil }
func (f *fakeExecNode) LoadConfig(confPath string, timeout int) ([]string, error) {
	return nil, nil
}
func (f *fakeExecNode) ExecCommand(cmd []string, in io.ReadCloser, out io.Writer, tty bool, ttyHeight uint, ttyWidth uint, resizeCh chan term.Winsize) error {
	return nil
}
func (f *fakeExecNode) ExecOnce(cmd []string) ([]byte, []byte, int, error) {
	f.gotCmd = cmd
	if f.execErr != nil {
		return nil, nil, 0, f.execErr
	}
	return f.stdout, f.stderr, f.exitCode, nil
}
func (f *fakeExecNode) GetConsoleCmd(shell bool) ([]string, error) { return nil, nil }
func (f *fakeExecNode) Capture(ifIndex int, out io.Writer) error   { return nil }
func (f *fakeExecNode) CopyFrom(srcPath, destPath string) error    { return nil }
func (f *fakeExecNode) CopyTo(srcPath, destPath string) error      { return nil }
func (f *fakeExecNode) ReadConfigFiles(confDir string, timeout int) (map[string][]byte, error) {
	return nil, nil
}
func (f *fakeExecNode) Save(dstPath string, timeout int) error { return nil }
func (f *fakeExecNode) GetInterfacesState() map[string]link.IfState {
	return nil
}
func (f *fakeExecNode) SetInterfaceState(ifIndex int, state link.IfState) error { return nil }
func (f *fakeExecNode) Close() error                                            { return nil }

func registerExecTestProject(t *testing.T, id string, node INetemNode) {
	t.Helper()
	openProjects[id] = &NetemProject{
		Id:   id,
		Name: "exec-test",
		Topology: &NetemTopologyManager{
			prjID: id,
			nodes: []NetemNode{{Instance: node}},
		},
	}
	t.Cleanup(func() { delete(openProjects, id) })
}

func execOnceClient(t *testing.T) proto.NetemClient {
	t.Helper()
	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer()))
	if err != nil {
		t.Fatalf("Unable to dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return proto.NewNetemClient(conn)
}

func TestNodeExecOnce_UnknownProject(t *testing.T) {
	client := execOnceClient(t)

	_, err := client.NodeExecOnce(context.Background(), &proto.NodeExecOnceRequest{
		PrjId: "no-such-project",
		Node:  "R1",
		Cmd:   []string{"vtysh", "-c", "show ip ospf"},
	})
	if err == nil {
		t.Fatal("expected an error for unknown project")
	}
	if !strings.Contains(err.Error(), "no-such-project") ||
		!strings.Contains(err.Error(), "not found") {
		t.Errorf("error should name the missing project, got: %v", err)
	}
}

func TestNodeExecOnce_UnknownNode(t *testing.T) {
	registerExecTestProject(t, "exec-unknown-node", &fakeExecNode{name: "R1", running: true})
	client := execOnceClient(t)

	_, err := client.NodeExecOnce(context.Background(), &proto.NodeExecOnceRequest{
		PrjId: "exec-unknown-node",
		Node:  "R42",
		Cmd:   []string{"vtysh", "-c", "show ip ospf"},
	})
	if err == nil {
		t.Fatal("expected an error for unknown node")
	}
	if !strings.Contains(err.Error(), "R42") {
		t.Errorf("error should name the missing node, got: %v", err)
	}
}

func TestNodeExecOnce_StoppedNode(t *testing.T) {
	registerExecTestProject(t, "exec-stopped", &fakeExecNode{name: "R1", running: false})
	client := execOnceClient(t)

	_, err := client.NodeExecOnce(context.Background(), &proto.NodeExecOnceRequest{
		PrjId: "exec-stopped",
		Node:  "R1",
		Cmd:   []string{"vtysh", "-c", "show ip ospf"},
	})
	if err == nil {
		t.Fatal("expected an error for a stopped node")
	}
	if !strings.Contains(err.Error(), "R1") || !strings.Contains(err.Error(), "not running") {
		t.Errorf("error should name the unavailable node, got: %v", err)
	}
}

func TestNodeExecOnce_CaptureAndNonZeroExit(t *testing.T) {
	node := &fakeExecNode{
		name:     "R1",
		running:  true,
		stdout:   []byte("% OSPF not enabled\n"),
		exitCode: 1,
	}
	registerExecTestProject(t, "exec-ok", node)
	client := execOnceClient(t)

	response, err := client.NodeExecOnce(context.Background(), &proto.NodeExecOnceRequest{
		PrjId: "exec-ok",
		Node:  "R1",
		Cmd:   []string{"vtysh", "-c", "show ip ospf"},
	})
	if err != nil {
		t.Fatalf("non-zero exit must not be a transport failure: %v", err)
	}
	if response.GetStatus().GetCode() != proto.StatusCode_OK {
		t.Errorf("status should be OK, got %v", response.GetStatus())
	}
	if string(response.GetStdout()) != "% OSPF not enabled\n" {
		t.Errorf("unexpected stdout: %q", response.GetStdout())
	}
	if response.GetExitCode() != 1 {
		t.Errorf("exit code should pass through as data, got %d", response.GetExitCode())
	}
	if len(node.gotCmd) != 3 || node.gotCmd[0] != "vtysh" {
		t.Errorf("node should receive the requested command, got %v", node.gotCmd)
	}
}

func TestNodeExecOnce_ExecTransportError(t *testing.T) {
	registerExecTestProject(t, "exec-transport", &fakeExecNode{
		name:    "R1",
		running: true,
		execErr: os.ErrClosed,
	})
	client := execOnceClient(t)

	_, err := client.NodeExecOnce(context.Background(), &proto.NodeExecOnceRequest{
		PrjId: "exec-transport",
		Node:  "R1",
		Cmd:   []string{"true"},
	})
	if err == nil {
		t.Fatal("expected a transport-level error when the exec itself fails")
	}
}
