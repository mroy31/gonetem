package server

import (
	"bytes"
	"context"
	"io"
	stdlog "log"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mroy31/gonetem/internal/options"
	"github.com/mroy31/gonetem/internal/proto"
	"github.com/mroy31/gonetem/internal/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
)

func dialer() func(context.Context, string) (net.Conn, error) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	proto.RegisterNetemServer(server, &netemServer{})

	go func() {
		if err := server.Serve(listener); err != nil {
			stdlog.Fatalf("Unable to launch server: %v", err)
		}
	}()

	return func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}
}

func TestServer_Version(t *testing.T) {
	options.InitServerConfig()
	ctx := context.Background()

	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer()))
	if err != nil {
		stdlog.Fatal(err)
	}
	defer conn.Close()

	client := proto.NewNetemClient(conn)
	response, err := client.ServerGetVersion(ctx, &emptypb.Empty{})
	if err != nil {
		t.Error("GetVersion method return an error", err)
	}

	if response.Status.GetCode() != proto.StatusCode_OK {
		t.Error("GetVersion.status is different from OK", response.GetStatus())
	}

	if response.GetVersion() != options.VERSION {
		t.Error("GetVersion - wrong version", response.GetVersion())
	} else {
		t.Log("GetVersion: success")
	}
}

func TestServer_Project(t *testing.T) {
	options.InitServerConfig()
	ctx := context.Background()

	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer()))
	if err != nil {
		stdlog.Fatal(err)
	}
	defer conn.Close()

	client := proto.NewNetemClient(conn)

	// get project list
	response, err := client.ProjectGetMany(ctx, &emptypb.Empty{})
	if err != nil {
		t.Errorf("GetProjects method return an error: %v", err)
	}
	if len(response.GetProjects()) != 0 {
		t.Errorf("Unexpected number of open projects: %d != 0", len(response.GetProjects()))
	}

	// create simple project
	prjPath := "/tmp/prjtest-archive.gnet"
	if err := createProject(prjPath, simpleNetwork); err != nil {
		t.Errorf("Unable to create .gnet file: %v", err)
		return
	}
	defer os.Remove(prjPath)
	data, err := os.ReadFile(prjPath)
	if err != nil {
		t.Errorf("Unable to open created .gnet file: %v", err)
		return
	}

	// open project
	openResponse, err := client.ProjectOpen(ctx, &proto.OpenRequest{
		Name: filepath.Base(prjPath),
		Data: data,
	})
	if err != nil {
		t.Errorf("OpenProject method return an error: %v", err)
		return
	}
	prjID := openResponse.GetId()

	// get project list
	response, err = client.ProjectGetMany(ctx, &emptypb.Empty{})
	if err != nil {
		t.Errorf("GetProjects method return an error: %v", err)
	}
	if len(response.GetProjects()) != 1 {
		t.Errorf("Unexpected number of open projects: %d != 1", len(response.GetProjects()))
	}

	// close project
	closeStream, err := client.ProjectClose(ctx, &proto.ProjectRequest{Id: prjID})
	if err != nil {
		t.Errorf("CloseProject method return an error: %v", err)
	}
	for {
		_, err := closeStream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			t.Errorf("ProjectClose stream return an error: %v", err)
			return
		}
	}
}

func TestServer_Save(t *testing.T) {
	options.InitServerConfig()
	ctx := context.Background()

	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer()))
	if err != nil {
		stdlog.Fatal(err)
	}
	defer conn.Close()

	client := proto.NewNetemClient(conn)
	// create simple project
	prjPath := "/tmp/prjtest-archive.gnet"
	if err := createProject(prjPath, simpleNetwork); err != nil {
		t.Errorf("Unable to create .gnet file: %v", err)
		return
	}
	defer os.Remove(prjPath)
	data, err := os.ReadFile(prjPath)
	if err != nil {
		t.Errorf("Unable to open created .gnet file: %v", err)
		return
	}

	// open project
	openResponse, err := client.ProjectOpen(ctx, &proto.OpenRequest{
		Name: filepath.Base(prjPath),
		Data: data,
	})
	if err != nil {
		t.Errorf("OpenProject method return an error: %v", err)
		return
	}
	prjID := openResponse.GetId()
	defer func() {
		closeStream, err := client.ProjectClose(ctx, &proto.ProjectRequest{Id: prjID})
		if err != nil {
			t.Errorf("CloseProject method return an error: %v", err)
		}
		for {
			_, err := closeStream.Recv()
			if err == io.EOF {
				return
			} else if err != nil {
				t.Errorf("ProjectClose stream return an error: %v", err)
				return
			}
		}
	}()

	// read network file
	netResponse, err := client.ReadNetworkFile(ctx, &proto.ProjectRequest{Id: prjID})
	if err != nil {
		t.Errorf("ReadNetworkFile method return an error: %v", err)
		return
	}
	if string(netResponse.GetData()) != simpleNetwork.network {
		t.Errorf("ReadNetworkFile: result different from expected network file")
	}

	// write network file
	newNetwork := `
nodes:
- name: R1
  type: docker.router
  ipv6: false
  mpls: false
`
	if _, err := client.WriteNetworkFile(ctx, &proto.WNetworkRequest{
		Id:   prjID,
		Data: []byte(newNetwork)}); err != nil {
		t.Errorf("WriteNetworkFile method return an error: %v", err)
		return
	}

	// check new network file
	netResponse, err = client.ReadNetworkFile(ctx, &proto.ProjectRequest{Id: prjID})
	if err != nil {
		t.Errorf("ReadNetworkFile method return an error: %v", err)
		return
	}
	if string(netResponse.GetData()) != newNetwork {
		t.Errorf("ReadNetworkFile: result different from expected network file")
	}

	// save new project
	saveStream, err := client.ProjectSave(ctx, &proto.ProjectRequest{Id: prjID})
	if err != nil {
		t.Errorf("SaveProject method return an error: %v", err)
		return
	}

	newPrjPath := "/tmp/prjtest-archive-new"
	for {
		msg, err := saveStream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			t.Errorf("SaveProject stream return an error: %v", err)
			return
		}

		switch msg.Code {
		case proto.ProjectSaveMsg_DATA:
			// check new project
			os.Mkdir(newPrjPath, 0755)
			defer os.RemoveAll(newPrjPath)

			if err := utils.OpenArchive(newPrjPath, bytes.NewReader(msg.GetData())); err != nil {
				saveStream.CloseSend()
				t.Errorf("Unable to extract saved project: %v", err)
				return
			}
		}
	}

	newNetworkData, err := os.ReadFile(path.Join(newPrjPath, networkFilename))
	if err != nil {
		t.Errorf("Unable to read new network file: %v", err)
		return
	}
	if string(newNetworkData) != newNetwork {
		t.Errorf("Saved network has not the expected content")
	}
}

func TestServer_ProjectGetStatus(t *testing.T) {
	options.InitServerConfig()
	ctx := context.Background()

	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer()))
	if err != nil {
		stdlog.Fatal(err)
	}
	defer conn.Close()

	client := proto.NewNetemClient(conn)

	// status of an unknown project must return an error
	if _, err := client.ProjectGetStatus(ctx, &proto.ProjectRequest{Id: "unknown-project-id"}); err == nil {
		t.Error("ProjectGetStatus method should return an error for an unknown project")
	}

	// create simple project
	prjPath := "/tmp/prjtest-archive.gnet"
	if err := createProject(prjPath, simpleNetwork); err != nil {
		t.Errorf("Unable to create .gnet file: %v", err)
		return
	}
	defer os.Remove(prjPath)
	data, err := os.ReadFile(prjPath)
	if err != nil {
		t.Errorf("Unable to open created .gnet file: %v", err)
		return
	}

	// open project
	openResponse, err := client.ProjectOpen(ctx, &proto.OpenRequest{
		Name: filepath.Base(prjPath),
		Data: data,
	})
	if err != nil {
		t.Errorf("OpenProject method return an error: %v", err)
		return
	}
	prjID := openResponse.GetId()
	defer func() {
		closeStream, err := client.ProjectClose(ctx, &proto.ProjectRequest{Id: prjID})
		if err != nil {
			t.Errorf("CloseProject method return an error: %v", err)
			return
		}
		for {
			_, err := closeStream.Recv()
			if err == io.EOF {
				return
			} else if err != nil {
				t.Errorf("ProjectClose stream return an error: %v", err)
				return
			}
		}
	}()

	// expected node names, full types and states (only launch nodes are started by TopologyRun)
	expectedNodes := map[string]struct {
		kind   string
		launch bool
	}{
		"switch":        {kind: "ovs", launch: true},
		"R1":            {kind: "docker.router", launch: true},
		"host":          {kind: "docker.host", launch: true},
		"hostNotLaunch": {kind: "docker.host", launch: false},
	}

	checkStatus := func(response *proto.StatusResponse, topoRunning bool) {
		t.Helper()

		if response.GetStatus().GetCode() != proto.StatusCode_OK {
			t.Errorf("ProjectGetStatus.status is different from OK: %v", response.GetStatus())
		}
		if response.GetName() != filepath.Base(prjPath) {
			t.Errorf("ProjectGetStatus: wrong project name %q != %q", response.GetName(), filepath.Base(prjPath))
		}
		if response.GetId() != prjID {
			t.Errorf("ProjectGetStatus: wrong project id %q != %q", response.GetId(), prjID)
		}
		if _, err := time.Parse("2006-01-02 15:04:05", response.GetOpenAt()); err != nil {
			t.Errorf("ProjectGetStatus: openAt has a wrong format: %q", response.GetOpenAt())
		}
		if response.GetRunning() != topoRunning {
			t.Errorf("ProjectGetStatus: running is %v != %v", response.GetRunning(), topoRunning)
		}

		if len(response.GetNodes()) != len(expectedNodes) {
			t.Errorf("ProjectGetStatus: unexpected number of nodes: %d != %d", len(response.GetNodes()), len(expectedNodes))
		}

		for _, node := range response.GetNodes() {
			expected, found := expectedNodes[node.GetName()]
			if !found {
				t.Errorf("ProjectGetStatus: unexpected node %s", node.GetName())
				continue
			}
			if node.GetType() != expected.kind {
				t.Errorf("ProjectGetStatus: node %s has wrong type %q != %q", node.GetName(), node.GetType(), expected.kind)
			}
			// only nodes with launch=true are started by TopologyRun
			expectedRunning := topoRunning && expected.launch
			if node.GetRunning() != expectedRunning {
				t.Errorf("ProjectGetStatus: node %s running is %v != %v", node.GetName(), node.GetRunning(), expectedRunning)
			}

			if !topoRunning {
				if len(node.GetInterfaces()) != 0 {
					t.Errorf("ProjectGetStatus: node %s should have no interface when topology is not running", node.GetName())
				}
			} else if node.GetRunning() {
				if len(node.GetInterfaces()) == 0 {
					t.Errorf("ProjectGetStatus: running node %s should have at least one interface", node.GetName())
				}
				for _, iface := range node.GetInterfaces() {
					if iface.GetName() == "" {
						t.Errorf("ProjectGetStatus: node %s has an interface with an empty name", node.GetName())
					}
					if iface.GetState() != proto.IfState_UP && iface.GetState() != proto.IfState_DOWN {
						t.Errorf("ProjectGetStatus: node %s interface %s has an unexpected state %v", node.GetName(), iface.GetName(), iface.GetState())
					}
				}
			}
		}
	}

	// check status of a stopped project
	response, err := client.ProjectGetStatus(ctx, &proto.ProjectRequest{Id: prjID})
	if err != nil {
		t.Errorf("ProjectGetStatus method return an error: %v", err)
		return
	}
	checkStatus(response, false)

	// run topology
	runStream, err := client.TopologyRun(ctx, &proto.ProjectRequest{Id: prjID})
	if err != nil {
		t.Errorf("TopologyRun method return an error: %v", err)
		return
	}
	for {
		_, err := runStream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			if strings.Contains(err.Error(), "operation not permitted") {
				t.Skipf("TopologyRun requires more privileges to create netns: %v", err)
			}
			t.Errorf("TopologyRun stream return an error: %v", err)
			return
		}
	}

	// check status of a running project
	response, err = client.ProjectGetStatus(ctx, &proto.ProjectRequest{Id: prjID})
	if err != nil {
		t.Errorf("ProjectGetStatus method return an error: %v", err)
		return
	}
	checkStatus(response, true)
}
