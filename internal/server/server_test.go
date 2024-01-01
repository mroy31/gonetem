package server

import (
	"bytes"
	"context"
	stdlog "log"
	"net"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/mroy31/gonetem/internal/options"
	"github.com/mroy31/gonetem/internal/proto"
	"github.com/mroy31/gonetem/internal/utils"
	"google.golang.org/grpc"
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

	conn, err := grpc.DialContext(ctx, "", grpc.WithInsecure(), grpc.WithContextDialer(dialer()))
	if err != nil {
		stdlog.Fatal(err)
	}
	defer conn.Close()

	client := proto.NewNetemClient(conn)
	response, err := client.GetVersion(ctx, &emptypb.Empty{})
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

	conn, err := grpc.DialContext(ctx, "", grpc.WithInsecure(), grpc.WithContextDialer(dialer()))
	if err != nil {
		stdlog.Fatal(err)
	}
	defer conn.Close()

	client := proto.NewNetemClient(conn)

	// get project list
	response, err := client.GetProjects(ctx, &emptypb.Empty{})
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
	openResponse, err := client.OpenProject(ctx, &proto.OpenRequest{
		Name: filepath.Base(prjPath),
		Data: data,
	})
	if err != nil {
		t.Errorf("OpenProject method return an error: %v", err)
		return
	}
	prjID := openResponse.GetId()

	// get project list
	response, err = client.GetProjects(ctx, &emptypb.Empty{})
	if err != nil {
		t.Errorf("GetProjects method return an error: %v", err)
	}
	if len(response.GetProjects()) != 1 {
		t.Errorf("Unexpected number of open projects: %d != 1", len(response.GetProjects()))
	}

	// close project
	_, err = client.CloseProject(ctx, &proto.ProjectRequest{Id: prjID})
	if err != nil {
		t.Errorf("CloseProject method return an error: %v", err)
	}
}

func TestServer_Save(t *testing.T) {
	options.InitServerConfig()
	ctx := context.Background()

	conn, err := grpc.DialContext(ctx, "", grpc.WithInsecure(), grpc.WithContextDialer(dialer()))
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
	openResponse, err := client.OpenProject(ctx, &proto.OpenRequest{
		Name: filepath.Base(prjPath),
		Data: data,
	})
	if err != nil {
		t.Errorf("OpenProject method return an error: %v", err)
		return
	}
	prjID := openResponse.GetId()
	defer client.CloseProject(ctx, &proto.ProjectRequest{Id: prjID})

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
	saveResponse, err := client.SaveProject(ctx, &proto.ProjectRequest{Id: prjID})
	if err != nil {
		t.Errorf("SaveProject method return an error: %v", err)
		return
	}
	// check new project
	newPrjPath := "/tmp/prjtest-archive-new"
	os.Mkdir(newPrjPath, 0755)
	defer os.RemoveAll(newPrjPath)
	if err := utils.OpenArchive(newPrjPath, bytes.NewReader(saveResponse.GetData())); err != nil {
		t.Errorf("Unable to extract saved project: %v", err)
		return
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
