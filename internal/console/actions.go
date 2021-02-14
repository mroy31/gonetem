package console

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/mroy31/gonetem/internal/proto"
	"github.com/mroy31/gonetem/internal/utils"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	networkFilename = "network.yml"
	emptyNetwork    = `
nodes:
`
)

func ListProjects(server string) *proto.PrjListResponse {
	client, err := NewClient(server)
	if err != nil {
		Fatal("Unable to connect to server: %v", err)
	}
	defer client.Conn.Close()

	projects, err := client.Client.GetProjects(context.Background(), &emptypb.Empty{})
	if err != nil {
		Fatal("Unable to get list of projects: %v", err)
	}

	if len(projects.GetProjects()) == 0 {
		fmt.Println(color.YellowString("No project open on the server"))
		return nil
	}

	return projects
}

func CreateProject(prjPath string) error {
	prj, err := os.Create(prjPath)
	if err != nil {
		return err
	}
	defer prj.Close()

	return utils.CreateOneFileArchive(prj, networkFilename, []byte(emptyNetwork))
}

func OpenProject(server, prjPath string) (string, error) {
	data, err := ioutil.ReadFile(prjPath)
	if err != nil {
		return "", err
	}

	client, err := NewClient(server)
	if err != nil {
		return "", fmt.Errorf("Server not responding")
	}
	defer client.Conn.Close()

	response, err := client.Client.OpenProject(context.Background(), &proto.OpenRequest{
		Name: filepath.Base(prjPath),
		Data: data,
	})
	if err != nil {
		return "", err
	}

	return response.GetId(), nil
}

func StartProject(server, prjID string) error {
	client, err := NewClient(server)
	if err != nil {
		return fmt.Errorf("Server not responding")
	}
	defer client.Conn.Close()

	if _, err := client.Client.Run(context.Background(), &proto.ProjectRequest{Id: prjID}); err != nil {
		return err
	}

	return nil
}
