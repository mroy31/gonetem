package console

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/google/shlex"
	"github.com/mroy31/gonetem/internal/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	RedPrintf     = color.New(color.FgRed).PrintfFunc()
	MagentaPrintf = color.New(color.FgMagenta).PrintfFunc()
)

func Fatal(msg string, a ...interface{}) {
	RedPrintf(msg+"\n", a...)
	os.Exit(1)
}

type NetemPrompt struct {
	server    string
	prjID     string
	prjPath   string
	processes []*exec.Cmd
}

func (p *NetemPrompt) Execute(s string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return
	}

	if s == "quit" || s == "exit" {
		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Prefix = "Close project " + filepath.Base(p.prjPath) + " : "
		s.Start()

		err := p.Close()
		s.Stop()
		if err != nil {
			RedPrintf(err.Error() + "\n")
		}

		os.Exit(0)
	}

	client, err := NewClient(p.server)
	if err != nil {
		RedPrintf("Unable to connect to gonetem server: %v", err)
		return
	}
	defer client.Conn.Close()

	args, err := shlex.Split(s)
	if err != nil {
		RedPrintf("Bad command line: %v", err)
		return
	}

	cmd := args[0]
	var cmdArgs []string
	if len(args) > 1 {
		cmdArgs = args[1:]
	}

	switch cmd {
	case "check":
		p.Check(client.Client, cmdArgs)

	case "console":
		p.Console(client.Client, cmdArgs)

	case "edit":
		p.Edit(client.Client)

	case "reload":
		p.Reload(client.Client)

	case "restart":
		p.Restart(client.Client, cmdArgs)

	case "run":
		p.Run(client.Client)

	case "save":
		p.Save(client.Client, p.prjPath)

	case "start":
		p.Start(client.Client, cmdArgs)

	case "stop":
		p.Stop(client.Client, cmdArgs)

	case "status":
		p.Status(client.Client)

	case "version":
		response, err := client.Client.GetVersion(context.Background(), &emptypb.Empty{})
		if err != nil {
			Fatal("Unable to get version: %v", err)
		}
		fmt.Println(response.GetVersion())

	default:
		fmt.Println("Unknown command, enter help for details")
	}
}

func (p *NetemPrompt) Check(client proto.NetemClient, cmdArgs []string) {
	if len(cmdArgs) != 0 {
		RedPrintf("check command does not take arguments\n")
		return
	}

	ack, err := client.Check(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		RedPrintf(err.Error() + "\n")
	} else {
		if ack.Status.Code == proto.StatusCode_OK {
			fmt.Println(color.GreenString("Network is OK"))
		} else {
			RedPrintf(ack.Status.Error + "\n")
		}
	}
}

func (p *NetemPrompt) Console(client proto.NetemClient, cmdArgs []string) {
	if len(cmdArgs) != 1 {
		RedPrintf("Wrong console invocation: console <node>\n")
		return
	}

	// search term command
	termPath, err := exec.LookPath("xterm")
	if err != nil {
		RedPrintf("xterm is not installed")
		return
	}

	node := fmt.Sprintf("%s.%s", p.prjID, cmdArgs[0])
	termArgs := []string{
		"-xrm", "XTerm.vt100.allowTitleOps: false",
		"-title", cmdArgs[0],
		"-e", "gonetem-console console " + node}
	cmd := exec.Command(termPath, termArgs...)
	if err := cmd.Start(); err != nil {
		RedPrintf("Error when starting console: %v", err)
		return
	}

	p.processes = append(p.processes, cmd)
}

func (p *NetemPrompt) Save(client proto.NetemClient, dstPath string) {
	response, err := client.SaveProject(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		RedPrintf("Unable to save project: %v\n", err)
		return
	}

	if dstPath == "" {
		RedPrintf("Project path is empty, set it in save command")
	} else if err := ioutil.WriteFile(dstPath, response.GetData(), 0644); err != nil {
		RedPrintf("Unable to write saved project to %s: %v", dstPath, err)
	}
}

func (p *NetemPrompt) Start(client proto.NetemClient, cmdArgs []string) {
	if len(cmdArgs) != 1 {
		RedPrintf("start command requires exactly 1 argument: <node>\n")
		return
	}

	ack, err := client.Start(context.Background(), &proto.NodeRequest{PrjId: p.prjID, Node: cmdArgs[0]})
	if err != nil {
		RedPrintf("Unable to start node: %v\n", err)
	} else {
		if ack.Status.Code == proto.StatusCode_ERROR {
			MagentaPrintf(ack.Status.Error + "\n")
		}
	}
}

func (p *NetemPrompt) Stop(client proto.NetemClient, cmdArgs []string) {
	if len(cmdArgs) != 1 {
		RedPrintf("stop command requires exactly 1 argument: <node>\n")
		return
	}

	ack, err := client.Stop(context.Background(), &proto.NodeRequest{PrjId: p.prjID, Node: cmdArgs[0]})
	if err != nil {
		RedPrintf("Unable to stop node: %v\n", err)
	} else {
		if ack.Status.Code == proto.StatusCode_ERROR {
			MagentaPrintf(ack.Status.Error + "\n")
		}
	}
}

func (p *NetemPrompt) Restart(client proto.NetemClient, cmdArgs []string) {
	if len(cmdArgs) != 1 {
		RedPrintf("restart command requires exactly 1 argument: <node>\n")
		return
	}

	ack, err := client.Restart(context.Background(), &proto.NodeRequest{PrjId: p.prjID, Node: cmdArgs[0]})
	if err != nil {
		RedPrintf("Unable to restart node: %v\n", err)
	} else {
		if ack.Status.Code == proto.StatusCode_ERROR {
			MagentaPrintf(ack.Status.Error + "\n")
		}
	}
}

func (p *NetemPrompt) Status(client proto.NetemClient) {
	response, err := client.GetProjectStatus(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		RedPrintf("Unable to get project status: %v\n", err)
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
				fmt.Print(color.MagentaString("Not Running\n"))
			}
		}
	}
}

func (p *NetemPrompt) Edit(client proto.NetemClient) {
	response, err := client.ReadNetworkFile(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		RedPrintf("Unable to get network file: %v\n", err)
		return
	}
	// write temp file for edition
	tempFilename := path.Join("/tmp", "gonetem-network-"+p.prjID)
	if err := ioutil.WriteFile(tempFilename, response.GetData(), 0644); err != nil {
		RedPrintf("Unable to write temp file for edition: %v\n", err)
		return
	}
	defer os.Remove(tempFilename)

	if err := EditFile(tempFilename, "vim"); err != nil {
		RedPrintf("Unable to edit temp file: %v\n", err)
		return
	}

	data, err := ioutil.ReadFile(tempFilename)
	if err != nil {
		RedPrintf("Unable to read edited network file: %v\n", err)
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
		RedPrintf("Unable to run the project: %v\n", err)
		return
	}
}

func (p *NetemPrompt) Reload(client proto.NetemClient) {
	if _, err := client.Reload(context.Background(), &proto.ProjectRequest{Id: p.prjID}); err != nil {
		RedPrintf("Unable to reload the project: %v\n", err)
		return
	}
}

func (p *NetemPrompt) Close() error {
	// First, stop all running processes (console, capture...)
	for _, cmd := range p.processes {
		done := make(chan interface{})
		go func() {
			done <- cmd.Wait()
		}()
		select {
		case <-time.After(100 * time.Millisecond):
			cmd.Process.Kill()
		case <-done:
		}
	}

	client, err := NewClient(p.server)
	if err != nil {
		return fmt.Errorf("Unable to connect to server: %v", err)
	} else {
		defer client.Conn.Close()

		_, err = client.Client.CloseProject(context.Background(), &proto.ProjectRequest{Id: p.prjID})
		if err != nil {
			return fmt.Errorf("Unable to close project: %v", err)
		}
	}

	return nil
}

func NewNetemPrompt(server, prjID, prjPath string) *NetemPrompt {
	return &NetemPrompt{server, prjID, prjPath, make([]*exec.Cmd, 0)}
}
