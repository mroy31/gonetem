package console

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/google/shlex"
	"github.com/mroy31/gonetem/internal/proto"
)

var (
	RedPrintf     = color.New(color.FgRed).PrintfFunc()
	MagentaPrintf = color.New(color.FgMagenta).PrintfFunc()
)

func Fatal(msg string, a ...interface{}) {
	RedPrintf(msg+"\n", a...)
	os.Exit(1)
}

func IsInterfaceId(arg string) bool {
	r, _ := regexp.Compile(`^\w+\.\d+$`)
	return r.MatchString(arg)
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
		s.Prefix = "Close project " + p.prjPath + " : "
		s.Start()

		err := p.Close()
		s.Stop()
		if err != nil {
			RedPrintf(err.Error() + "\n")
		}

		os.Exit(0)
	}

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
	case "capture":
		p.Capture(cmdArgs)

	case "check":
		p.execWithClient(cmdArgs, 0, p.Check)

	case "console":
		p.execWithClient(cmdArgs, 1, p.Console)

	case "edit":
		p.execWithClient(cmdArgs, 0, p.Edit)

	case "ifState":
		p.execWithClient(cmdArgs, 2, p.IfState)

	case "reload":
		p.execWithClient(cmdArgs, 0, p.Reload)

	case "restart":
		p.execWithClient(cmdArgs, 1, p.Restart)

	case "run":
		p.execWithClient(cmdArgs, 0, p.Run)

	case "save":
		p.execWithClient(cmdArgs, 0, p.Save)

	case "saveAs":
		p.execWithClient(cmdArgs, 1, p.SaveAs)

	case "start":
		p.execWithClient(cmdArgs, 1, p.Start)

	case "stop":
		p.execWithClient(cmdArgs, 1, p.Stop)

	case "status":
		p.execWithClient(cmdArgs, 0, p.Status)

	default:
		fmt.Println("Unknown command, enter help for details")
	}
}

func (p *NetemPrompt) execWithClient(cmdArgs []string, nbArgs int, execFunc func(client proto.NetemClient, cmdArgs []string)) {
	if len(cmdArgs) != nbArgs {
		RedPrintf("Wrong invocation: %d arguments expected for this command\n", nbArgs)
		return
	}

	client, err := NewClient(p.server)
	if err != nil {
		RedPrintf("Unable to connect to gonetem server: %v", err)
		return
	}
	defer client.Conn.Close()

	execFunc(client.Client, cmdArgs)
}

func (p *NetemPrompt) Capture(cmdArgs []string) {
	if len(cmdArgs) != 1 {
		RedPrintf("Wrong capture invocation: capture <node>.<ifIndex>\n")
		return
	}

	args := strings.Split(cmdArgs[0], ".")
	if len(args) != 2 {
		RedPrintf("Wrong interface identifier: <node>.<ifIndex> expected\n")
		return
	}
	ifIndex, err := strconv.Atoi(args[1])
	if err != nil {
		RedPrintf("ifIndex is not a number\n")
		return
	}

	// Check wireshark is present
	wiresharkPath, err := exec.LookPath("wireshark")
	if err != nil {
		RedPrintf("wireshark is not installed\n")
		return
	}

	client, err := NewClient(p.server)
	if err != nil {
		RedPrintf("Unable to connect to gonetem server: %v", err)
		return
	}

	stream, err := client.Client.Capture(context.Background(), &proto.NodeInterfaceRequest{
		PrjId:   p.prjID,
		Node:    args[0],
		IfIndex: int32(ifIndex),
	})
	if err != nil {
		RedPrintf("Error when start capturing %v\n", err)
		return
	}

	msg, err := stream.Recv()
	if err != nil {
		RedPrintf("Error when start capturing %v\n", err)
		return
	}

	if msg.GetCode() == proto.CaptureSrvMsg_ERROR {
		RedPrintf("%s\n", string(msg.GetData()))
		return
	}

	rIn, wIn := io.Pipe()
	captureArgs := []string{
		"-o", "'gui.window_title:" + cmdArgs[0] + "'",
		"-k", "-i", "-"}
	cmd := exec.Command(wiresharkPath, captureArgs...)
	cmd.Stdin = rIn

	if err := cmd.Start(); err != nil {
		RedPrintf("Error when starting wireshark: %v\n", err)
		return
	}

	go func() {
		defer client.Conn.Close()

		for {
			msg, err := stream.Recv()
			if err != nil {
				return
			}

			switch msg.GetCode() {
			case proto.CaptureSrvMsg_STDOUT:
				wIn.Write(msg.GetData())
			case proto.CaptureSrvMsg_ERROR:
				RedPrintf("Error when capturing wireshark: %s\n", string(msg.GetData()))
				cmd.Process.Signal(os.Kill)
				return
			}
		}

	}()
}

func (p *NetemPrompt) IfState(client proto.NetemClient, cmdArgs []string) {
	if !IsInterfaceId(cmdArgs[0]) {
		RedPrintf("Interface identifier is not valid: <node>.<ifIndex> expected\n")
		return
	}

	state, found := map[string]proto.IfState{
		"up":   proto.IfState_UP,
		"down": proto.IfState_DOWN,
	}[cmdArgs[1]]
	if !found {
		RedPrintf("State is not valid: up|down expected\n")
		return
	}

	ifArgs := strings.Split(cmdArgs[0], ".")
	ifIndex, _ := strconv.Atoi(ifArgs[1])

	_, err := client.SetIfState(
		context.Background(),
		&proto.NodeIfStateRequest{
			PrjId:   p.prjID,
			Node:    ifArgs[0],
			IfIndex: int32(ifIndex),
			State:   state,
		})
	if err != nil {
		RedPrintf("Unable to change interface state: %v\n", err)
	}
}

func (p *NetemPrompt) Check(client proto.NetemClient, cmdArgs []string) {
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
	// first check that we can run console for this node
	ack, err := client.CanRunConsole(context.Background(), &proto.NodeRequest{
		PrjId: p.prjID,
		Node:  cmdArgs[0],
	})
	if err != nil {
		RedPrintf(err.Error() + "\n")
		return
	} else if ack.GetStatus().GetCode() == proto.StatusCode_ERROR {
		RedPrintf(ack.GetStatus().GetError() + "\n")
		return
	}

	// search term command
	termPath, err := exec.LookPath("xterm")
	if err != nil {
		RedPrintf("xterm is not installed\n")
		return
	}

	node := fmt.Sprintf("%s.%s", p.prjID, cmdArgs[0])
	termArgs := []string{
		"-xrm", "XTerm.vt100.allowTitleOps: false",
		"-title", cmdArgs[0],
		"-e", "gonetem-console console " + node}
	cmd := exec.Command(termPath, termArgs...)
	if err := cmd.Start(); err != nil {
		RedPrintf("Error when starting console: %v\n", err)
		return
	}

	p.processes = append(p.processes, cmd)
}

func (p *NetemPrompt) save(client proto.NetemClient, dstPath string) {
	response, err := client.SaveProject(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		RedPrintf("Unable to save project: %v\n", err)
		return
	}

	if err := ioutil.WriteFile(dstPath, response.GetData(), 0644); err != nil {
		RedPrintf("Unable to write saved project to %s: %v\n", dstPath, err)
	}
}

func (p *NetemPrompt) Save(client proto.NetemClient, cmdArgs []string) {
	if p.prjPath == "" {
		RedPrintf("Project path is empty, use saveAs command if you connect to running project\n")
		return
	}

	p.save(client, p.prjPath)
}

func (p *NetemPrompt) SaveAs(client proto.NetemClient, cmdArgs []string) {
	p.save(client, cmdArgs[0])
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

func (p *NetemPrompt) Status(client proto.NetemClient, cmdArgs []string) {
	response, err := client.GetProjectStatus(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		RedPrintf("Unable to get project status: %v\n", err)
		return
	}

	fmt.Println("Project " + response.GetName())
	fmt.Println("- Id: " + response.GetId())
	fmt.Println("- OpenAt: " + response.GetOpenAt())
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
				for _, i := range nodeInfo.GetInterfaces() {
					fmt.Print("     - " + i.GetName() + ": ")
					switch i.GetState() {
					case proto.IfState_DOWN:
						fmt.Print(color.MagentaString("Down\n"))
					case proto.IfState_UP:
						fmt.Print(color.GreenString("Up\n"))
					}
				}
			} else {
				fmt.Print(color.MagentaString("Not Running\n"))
			}
		}
	}
}

func (p *NetemPrompt) Edit(client proto.NetemClient, cmdArgs []string) {
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

func (p *NetemPrompt) Run(client proto.NetemClient, cmdArgs []string) {
	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Prefix = "Run project " + p.prjPath + " : "
	s.Start()

	_, err := client.Run(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	s.Stop()

	if err != nil {
		RedPrintf("Unable to run the project: %v\n", err)
		return
	}
}

func (p *NetemPrompt) Reload(client proto.NetemClient, cmdArgs []string) {
	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Prefix = "Reload project " + p.prjPath + " : "
	s.Start()

	_, err := client.Reload(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	s.Stop()

	if err != nil {
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
