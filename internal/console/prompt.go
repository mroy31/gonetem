package console

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/fatih/color"
	"github.com/google/shlex"
	"github.com/mroy31/gonetem/internal/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	redPrintf = color.New(color.FgRed).PrintfFunc()
)

func Fatal(msg string, a ...interface{}) {
	redPrintf(msg+"\n", a...)
	os.Exit(1)
}

type NetemPrompt struct {
	server  string
	prjID   string
	prjPath string
}

func (p *NetemPrompt) Execute(s string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return
	}

	if s == "quit" || s == "exit" {
		p.Close()
	}

	client, err := NewClient(p.server)
	if err != nil {
		Fatal("Unable to connect to gonetem server: %v", err)
	}
	defer client.Conn.Close()

	switch s {
	case "edit":
		p.Edit(client.Client)

	case "reload":
		p.Reload(client.Client)

	case "run":
		p.Run(client.Client)

	case "save":
		p.Save(client.Client, p.prjPath)

	case "status":
		p.Status(client.Client)

	case "version":
		response, err := client.Client.GetVersion(context.Background(), &emptypb.Empty{})
		if err != nil {
			Fatal("Unable to get version: %v", err)
		}
		fmt.Println(response.GetVersion())
	default:
		_, err := shlex.Split(s)
		if err != nil {
			Fatal("Unable to parse command line: %v", err)
		}

		fmt.Println("Unknown command, enter help for details")
	}
}

func (p *NetemPrompt) Save(client proto.NetemClient, dstPath string) {
	response, err := client.SaveProject(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		redPrintf("Unable to save project: %v\n", err)
		return
	}

	if dstPath == "" {
		redPrintf("Project path is empty, set it in save command")
	} else if err := ioutil.WriteFile(dstPath, response.GetData(), 0644); err != nil {
		redPrintf("Unable to write saved project to %s: %v", dstPath, err)
	}
}

func (p *NetemPrompt) Status(client proto.NetemClient) {
	response, err := client.GetProjectStatus(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		redPrintf("Unable to get project status: %v\n", err)
		return
	}

	fmt.Println("Project " + response.GetName())
	fmt.Print("- State: ")
	if response.GetRunning() {
		fmt.Print(color.GreenString("Running\n"))
	} else {
		fmt.Print(color.YellowString("Not Running\n"))
	}

	if len(response.GetNodes()) > 0 {
		fmt.Println("- Nodes:")
		for _, nodeInfo := range response.GetNodes() {
			fmt.Print("   - " + nodeInfo.GetName() + ": ")
			if nodeInfo.GetRunning() {
				fmt.Print(color.GreenString("Running\n"))
			} else {
				fmt.Print(color.YellowString("Not Running\n"))
			}
		}
	}
}

func (p *NetemPrompt) Edit(client proto.NetemClient) {
	response, err := client.ReadNetworkFile(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		redPrintf("Unable to get network file: %v\n", err)
		return
	}
	// write temp file for edition
	tempFilename := path.Join("/tmp", "gonetem-network-"+p.prjID)
	if err := ioutil.WriteFile(tempFilename, response.GetData(), 0644); err != nil {
		redPrintf("Unable to write temp file for edition: %v\n", err)
		return
	}
	defer os.Remove(tempFilename)

	if err := EditFile(tempFilename, "vim"); err != nil {
		redPrintf("Unable to edit temp file: %v\n", err)
		return
	}

	data, err := ioutil.ReadFile(tempFilename)
	if err != nil {
		redPrintf("Unable to read edited network file: %v\n", err)
		return
	}

	if _, err = client.WriteNetworkFile(context.Background(), &proto.WNetworkRequest{
		Id:   p.prjID,
		Data: data,
	}); err != nil {
		Fatal("Unable to write modified network file on server: %v\n", err)
	}
}

func (p *NetemPrompt) Run(client proto.NetemClient) {
	if _, err := client.Run(context.Background(), &proto.ProjectRequest{Id: p.prjID}); err != nil {
		redPrintf("Unable to run the project: %v\n", err)
		return
	}
}

func (p *NetemPrompt) Reload(client proto.NetemClient) {
	if _, err := client.Reload(context.Background(), &proto.ProjectRequest{Id: p.prjID}); err != nil {
		redPrintf("Unable to reload the project: %v\n", err)
		return
	}
}

func (p *NetemPrompt) Close() {
	client, err := NewClient(p.server)
	if err != nil {
		redPrintf("Unable to connect to gonetem server: %v", err)
	} else {
		_, err = client.Client.CloseProject(context.Background(), &proto.ProjectRequest{Id: p.prjID})
		if err != nil {
			redPrintf("Unable to close project: %v", err)
		}
	}
	defer client.Conn.Close()

	fmt.Println("Bye!")
	os.Exit(0)
}

func NewNetemPrompt(server, prjID, prjPath string) *NetemPrompt {
	return &NetemPrompt{server, prjID, prjPath}
}
