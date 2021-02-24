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

type NetemCommand struct {
	Desc  string
	Usage string
	Args  []string
	Run   func(p *NetemPrompt, cmdArgs []string)
}

type NetemPrompt struct {
	server    string
	prjID     string
	prjPath   string
	processes []*exec.Cmd
	commands  map[string]*NetemCommand
}

func (p *NetemPrompt) RegisterCommands() {
	p.commands["capture"] = &NetemCommand{
		Desc:  "Capture trafic on an interface",
		Usage: "capture <node_name>.<if_number>",
		Args:  []string{`^\w+\.\d+$`},
		Run: func(p *NetemPrompt, cmdArgs []string) {
			p.Capture(cmdArgs)
		},
	}
	p.commands["check"] = &NetemCommand{
		Desc:  "Check that the topology file is correct. If not, return found errors",
		Usage: "check",
		Args:  []string{},
		Run: func(p *NetemPrompt, cmdArgs []string) {
			p.execWithClient(cmdArgs, p.Check)
		},
	}
	p.commands["console"] = &NetemCommand{
		Desc:  "Open a console for a node",
		Usage: "console <node_name>",
		Args:  []string{`^\w+$`},
		Run: func(p *NetemPrompt, cmdArgs []string) {
			p.execWithClient(cmdArgs, func(client proto.NetemClient, cmdArgs []string) {
				p.startConsole(client, cmdArgs[0], false)
			})
		},
	}
	p.commands["edit"] = &NetemCommand{
		Desc:  "Edit the topology",
		Usage: "edit",
		Args:  []string{},
		Run: func(p *NetemPrompt, cmdArgs []string) {
			p.execWithClient(cmdArgs, p.Edit)
		},
	}
	p.commands["ifState"] = &NetemCommand{
		Desc:  "Enable/disable a node interface",
		Usage: "ifState <node_name>.<if_number> up|down",
		Args:  []string{`^\w+\.\d+$`, `^up|down$`},
		Run: func(p *NetemPrompt, cmdArgs []string) {
			p.execWithClient(cmdArgs, p.IfState)
		},
	}
	p.commands["reload"] = &NetemCommand{
		Desc:  "Reload the project",
		Usage: "reload",
		Args:  []string{},
		Run: func(p *NetemPrompt, cmdArgs []string) {
			p.execWithClient(cmdArgs, p.Reload)
		},
	}
	p.commands["restart"] = &NetemCommand{
		Desc:  "Restart a node",
		Usage: "restart <node_name>",
		Args:  []string{`^\w+$`},
		Run: func(p *NetemPrompt, cmdArgs []string) {
			p.execWithClient(cmdArgs, p.Restart)
		},
	}
	p.commands["run"] = &NetemCommand{
		Desc:  "Start the project",
		Usage: "run",
		Args:  []string{},
		Run: func(p *NetemPrompt, cmdArgs []string) {
			p.execWithClient(cmdArgs, p.Run)
		},
	}
	p.commands["save"] = &NetemCommand{
		Desc:  "Save the project",
		Usage: "save",
		Args:  []string{},
		Run: func(p *NetemPrompt, cmdArgs []string) {
			p.execWithClient(cmdArgs, p.Save)
		},
	}
	p.commands["saveAs"] = &NetemCommand{
		Desc:  "Save the project in a new file",
		Usage: "saveAs <project_path>/<name>.gnet",
		Args:  []string{`^.*\.gnet$`},
		Run: func(p *NetemPrompt, cmdArgs []string) {
			p.execWithClient(cmdArgs, p.SaveAs)
		},
	}
	p.commands["shell"] = &NetemCommand{
		Desc:  "Open a shell console for a node",
		Usage: "shell <node_name>",
		Args:  []string{`^\w+$`},
		Run: func(p *NetemPrompt, cmdArgs []string) {
			p.execWithClient(cmdArgs, func(client proto.NetemClient, cmdArgs []string) {
				p.startConsole(client, cmdArgs[0], true)
			})
		},
	}
	p.commands["start"] = &NetemCommand{
		Desc:  "Start a node",
		Usage: "start <node_name>",
		Args:  []string{`^\w+$`},
		Run: func(p *NetemPrompt, cmdArgs []string) {
			p.execWithClient(cmdArgs, p.Start)
		},
	}
	p.commands["stop"] = &NetemCommand{
		Desc:  "Stop a node",
		Usage: "stop <node_name>",
		Args:  []string{`^\w+$`},
		Run: func(p *NetemPrompt, cmdArgs []string) {
			p.execWithClient(cmdArgs, p.Stop)
		},
	}
	p.commands["status"] = &NetemCommand{
		Desc:  "Display the state of the project",
		Usage: "status",
		Args:  []string{},
		Run: func(p *NetemPrompt, cmdArgs []string) {
			p.execWithClient(cmdArgs, p.Status)
		},
	}
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

	if s == "help" {
		for name, cmd := range p.commands {
			fmt.Println(color.BlueString(name))
			fmt.Println("  " + cmd.Desc)
			fmt.Println("  Usage: " + cmd.Desc)
		}
		return
	}

	args, err := shlex.Split(s)
	if err != nil {
		RedPrintf("Bad command line: %v", err)
		return
	}

	cmd, found := p.commands[args[0]]
	if !found {
		RedPrintf("Unknown command, enter help for details\n")
		return
	}

	// check args
	if len(args) != len(cmd.Args)+1 {
		RedPrintf("Wrong number of arguments for '%s'\n\tusage: %s\n", args[0], cmd.Usage)
		return
	}
	for idx, argRe := range cmd.Args {
		r, _ := regexp.Compile(argRe)
		if !r.MatchString(args[idx+1]) {
			RedPrintf("Wrong format for argument %d\n\tusage: %s\n", idx+1, cmd.Usage)
			return
		}
	}

	// run the command
	cmd.Run(p, args[1:])
}

func (p *NetemPrompt) execWithClient(cmdArgs []string, execFunc func(client proto.NetemClient, cmdArgs []string)) {
	client, err := NewClient(p.server)
	if err != nil {
		RedPrintf("Unable to connect to gonetem server: %v", err)
		return
	}
	defer client.Conn.Close()

	execFunc(client.Client, cmdArgs)
}

func (p *NetemPrompt) Capture(cmdArgs []string) {
	args := strings.Split(cmdArgs[0], ".")
	ifIndex, _ := strconv.Atoi(args[1])

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

func (p *NetemPrompt) startConsole(client proto.NetemClient, nodeName string, shell bool) {
	// first check that we can run console for this node
	ack, err := client.CanRunConsole(context.Background(), &proto.NodeRequest{
		PrjId: p.prjID,
		Node:  nodeName,
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

	node := fmt.Sprintf("%s.%s", p.prjID, nodeName)
	shellOpt := ""
	if shell {
		shellOpt = "--shell "
	}

	termArgs := []string{
		"-xrm", "XTerm.vt100.allowTitleOps: false",
		"-title", nodeName,
		"-e", "gonetem-console console " + shellOpt + node}
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
	p := &NetemPrompt{server, prjID, prjPath, make([]*exec.Cmd, 0), make(map[string]*NetemCommand)}
	p.RegisterCommands()

	return p
}
