package console

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/docker/docker/pkg/system"
	"github.com/fatih/color"
	"github.com/google/shlex"
	"github.com/mroy31/gonetem/internal/options"
	"github.com/mroy31/gonetem/internal/proto"
	"github.com/mroy31/gonetem/internal/utils"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

type copyDirection int

const (
	fromNode copyDirection = 1 << iota
	toNode
	acrossNodes = fromNode | toNode
)

var (
	RedPrintf     = color.New(color.FgRed).PrintfFunc()
	MagentaPrintf = color.New(color.FgMagenta).PrintfFunc()
)

func Fatal(msg string, a ...interface{}) {
	RedPrintf(msg+"\n", a...)
	os.Exit(1)
}

func splitCopyArg(arg string) (container, path string) {
	if system.IsAbs(arg) {
		return "", arg
	}

	parts := strings.SplitN(arg, ":", 2)

	if len(parts) == 1 || strings.HasPrefix(parts[0], ".") {
		// Either there's no `:` in the arg
		// OR it's an explicit local relative path like `./file:name.txt`.
		return "", arg
	}

	return parts[0], parts[1]
}

type NetemNode struct {
	Name       string
	Interfaces []string
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
	nodes     []NetemNode // use by completion
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
	p.commands["config"] = &NetemCommand{
		Desc:  "Save the configuration files in the specified folder",
		Usage: "config <dest_path>",
		Args:  []string{`^.+$`},
		Run: func(p *NetemPrompt, cmdArgs []string) {
			p.execWithClient(cmdArgs, p.Config)
		},
	}
	p.commands["console"] = &NetemCommand{
		Desc:  "Open a console for a node",
		Usage: "console <node_name>",
		Args:  []string{`^\w+$`},
		Run: func(p *NetemPrompt, cmdArgs []string) {
			p.execWithClient(cmdArgs, func(client proto.NetemClient, cmdArgs []string) {
				if cmdArgs[0] == "all" {
					p.startConsoleAll(client, false)
				} else {
					p.startConsole(client, cmdArgs[0], false)
				}
			})
		},
	}
	p.commands["copy"] = &NetemCommand{
		Desc:  "Copy a file from/to a node",
		Usage: "copy sourceFile <node>:destFile",
		Args:  []string{`^.+$`, `^.+$`},
		Run: func(p *NetemPrompt, cmdArgs []string) {
			srcNode, srcPath := splitCopyArg(cmdArgs[0])
			destNode, destPath := splitCopyArg(cmdArgs[1])

			var direction copyDirection
			if srcNode != "" {
				direction |= fromNode
			}
			if destNode != "" {
				direction |= toNode
			}

			switch direction {
			case fromNode:
				p.CopyFrom(srcNode, srcPath, destPath)
			case toNode:
				p.CopyTo(srcPath, destNode, destPath)
			case acrossNodes:
				RedPrintf("copying between containers is not supported\n")
			default:
				RedPrintf("must specify at least one container source\n")
			}
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
			// node list needs to be updated for completion
			p.refreshNodeList()
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
			// node list needs to be updated for completion
			p.refreshNodeList()
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
				if cmdArgs[0] == "all" {
					p.startConsoleAll(client, true)
				} else {
					p.startConsole(client, cmdArgs[0], true)
				}
			})
		},
	}
	p.commands["start"] = &NetemCommand{
		Desc:  "Start a node",
		Usage: "start <node_name>",
		Args:  []string{`^\w+$`},
		Run: func(p *NetemPrompt, cmdArgs []string) {
			if cmdArgs[0] == "all" {
				p.execWithClient(cmdArgs, p.StartAll)
			} else {
				p.execWithClient(cmdArgs, p.Start)
			}
		},
	}
	p.commands["stop"] = &NetemCommand{
		Desc:  "Stop a node",
		Usage: "stop <node_name>",
		Args:  []string{`^\w+$`},
		Run: func(p *NetemPrompt, cmdArgs []string) {
			if cmdArgs[0] == "all" {
				p.execWithClient(cmdArgs, p.StopAll)
			} else {
				p.execWithClient(cmdArgs, p.Stop)
			}
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

func (p *NetemPrompt) refreshNodeList() {
	client, err := NewClient(p.server)
	if err != nil {
		return
	}
	defer client.Conn.Close()

	status, err := client.Client.GetProjectStatus(
		context.Background(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		return
	}

	p.nodes = make([]NetemNode, 0)
	for _, node := range status.GetNodes() {
		nStatus := NetemNode{
			Name:       node.GetName(),
			Interfaces: make([]string, 0),
		}
		for _, ifStatus := range node.GetInterfaces() {
			nStatus.Interfaces = append(nStatus.Interfaces, ifStatus.Name)
		}

		p.nodes = append(p.nodes, nStatus)
	}
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

func (p *NetemPrompt) getCancelContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)

	signal.Notify(c, os.Interrupt)

	go func() {
		select {
		case <-c:
			cancel()
		case <-ctx.Done():
			return
		}
	}()

	return ctx
}

func (p *NetemPrompt) CopyFrom(srcNode, srcPath, destPath string) {
	file, err := os.Create(destPath)
	if err != nil {
		RedPrintf("Unable to create/open %s: %v\n", destPath, err)
	}
	defer file.Close()

	client, err := NewClient(p.server)
	if err != nil {
		RedPrintf("Unable to connect to gonetem server: %v\n", err)
		return
	}
	defer client.Conn.Close()

	stream, err := client.Client.CopyFrom(context.Background(), &proto.CopyMsg{
		Code:     proto.CopyMsg_INIT,
		PrjId:    p.prjID,
		Node:     srcNode,
		NodePath: srcPath,
	})
	if err != nil {
		RedPrintf("CopyFrom %s:%s returns an error: %v\n", srcNode, srcPath, err)
		return
	}

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			RedPrintf("%v\n", err)
			return
		}

		switch msg.GetCode() {
		case proto.CopyMsg_DATA:
			file.Write(msg.GetData())
		case proto.CopyMsg_ERROR:
			RedPrintf("CopyFrom %s:%s returns an error: %s\n", srcNode, srcPath, string(msg.GetData()))
		}
	}
}

func (p *NetemPrompt) CopyTo(srcPath, destNode, destPath string) {
	stat, err := os.Stat(srcPath)
	if os.IsNotExist(err) {
		RedPrintf("File %s does not exist\n", srcPath)
		return
	} else if err != nil {
		RedPrintf("Unable to stat %s: %v\n", srcPath, err)
		return
	}

	// check it is a regular file
	if !stat.Mode().IsRegular() {
		RedPrintf("%s is not a regular file\n", srcPath)
		return
	}

	client, err := NewClient(p.server)
	if err != nil {
		RedPrintf("Unable to connect to gonetem server: %v\n", err)
		return
	}
	defer client.Conn.Close()

	buffer := make([]byte, 1024)
	file, err := os.Open(srcPath)
	if err != nil {
		RedPrintf("Unable to open %s: %v\n", srcPath, err)
		return
	}
	defer file.Close()

	stream, err := client.Client.CopyTo(context.Background())
	if err != nil {
		RedPrintf("CopyTo: %v\n", err)
		return
	}

	if err := stream.Send(&proto.CopyMsg{
		Code:     proto.CopyMsg_INIT,
		PrjId:    p.prjID,
		Node:     destNode,
		NodePath: destPath,
	}); err != nil {
		RedPrintf("Unable to init CopyTo: %v\n", err)
		return
	}

	for {
		n, err := file.Read(buffer)
		if err != nil {
			break
		}

		stream.Send(&proto.CopyMsg{
			Code: proto.CopyMsg_DATA,
			Data: buffer[:n],
		})
	}

	ack, err := stream.CloseAndRecv()
	if err != nil {
		RedPrintf("Error in CopyTo cmd: %v\n", err)
		return
	} else if ack.GetStatus().GetCode() == proto.StatusCode_ERROR {
		RedPrintf("CopyTo returns an error: %s\n", string(ack.Status.Error))
	}
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
		"-o", fmt.Sprintf("gui.window_title:%s@%s", args[1], args[0]),
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

func (p *NetemPrompt) Config(client proto.NetemClient, cmdArgs []string) {
	dstPath := cmdArgs[0]
	stat, err := os.Stat(dstPath)
	if err != nil {
		RedPrintf("Unable to get stat on dest path '%s'\n\t%v\n", dstPath, err)
		return
	} else if !stat.IsDir() {
		RedPrintf("Dest path '%s' is not a directory\n", dstPath)
		return
	}

	response, err := client.GetProjectConfigs(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		RedPrintf("Unable to get project configuration files: %v\n", err)
		return
	}

	buffer := bytes.NewBuffer(response.GetData())
	if err := utils.OpenArchive(dstPath, buffer); err != nil {
		RedPrintf("Unable to extract configuration files: %v\n", err)
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

	node := fmt.Sprintf("%s.%s", p.prjID, nodeName)
	shellOpt := ""
	if shell {
		shellOpt = "--shell "
	}
	consoleCmd := struct {
		Name string
		Cmd  string
	}{Name: nodeName, Cmd: "gonetem-console console " + shellOpt + node}

	var buf bytes.Buffer
	tmpl, err := template.New("terminal").Parse(options.ConsoleConfig.Terminal)
	if err != nil {
		RedPrintf("Unable to parse terminal line in config file: %v", err)
		return
	}
	err = tmpl.Execute(&buf, consoleCmd)
	if err != nil {
		RedPrintf("Unable to parse terminal line in config file: %v", err)
		return
	}

	args, err := shlex.Split(buf.String())
	if err != nil {
		RedPrintf("Bad command line: %v", err)
		return
	}

	// search term command
	termPath, err := exec.LookPath(args[0])
	if err != nil {
		RedPrintf("terminal '%s' is not installed\n", args[0])
		return
	}

	cmd := exec.Command(termPath, args[1:]...)
	if err := cmd.Start(); err != nil {
		RedPrintf("Error when starting console: %v\n", err)
		return
	}

	p.processes = append(p.processes, cmd)
}

func (p *NetemPrompt) startConsoleAll(client proto.NetemClient, shell bool) {
	response, err := client.GetProjectStatus(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		RedPrintf("Unable to get project status: %v\n", err)
		return
	}

	for _, node := range response.GetNodes() {
		ack, err := client.CanRunConsole(context.Background(), &proto.NodeRequest{
			PrjId: p.prjID,
			Node:  node.GetName(),
		})
		if err != nil || ack.GetStatus().GetCode() == proto.StatusCode_ERROR {
			continue
		}

		p.startConsole(client, node.GetName(), shell)
	}
}

func (p *NetemPrompt) save(client proto.NetemClient, dstPath string) {
	stream, err := client.ProjectSave(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		RedPrintf("Unable to save project: %v\n", err)
		return
	}

	mpBar := mpb.New(mpb.WithWidth(48))
	bars := make([]ProgressBarT, 1)

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			ProgressForceComplete(bars)
			break
		} else if err != nil {
			ProgressAbort(bars, true)
			RedPrintf("Unable to save project: %v\n", err)
			return
		}

		switch msg.Code {
		case proto.ProjectSaveMsg_NODE_COUNT:
			bars[0] = ProgressBarT{
				Total: int(msg.Total),
				Bar: mpBar.AddBar(int64(msg.Total),
					mpb.BarRemoveOnComplete(),
					mpb.PrependDecorators(decor.Counters(0, "Save nodes: %d/%d")),
				),
			}

		case proto.ProjectSaveMsg_NODE_SAVE:
			bars[0].Bar.Increment()

		case proto.ProjectSaveMsg_DATA:
			ProgressForceComplete(bars)
			if err := os.WriteFile(dstPath, msg.GetData(), 0644); err != nil {
				RedPrintf("Unable to write saved project to %s: %v\n", dstPath, err)
			}
		}
	}

	mpBar.Wait()
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
	ack, err := client.Start(p.getCancelContext(), &proto.NodeRequest{PrjId: p.prjID, Node: cmdArgs[0]})
	if err != nil {
		RedPrintf("Unable to start node: %v\n", err)
	} else {
		if ack.Status.Code == proto.StatusCode_ERROR {
			MagentaPrintf(ack.Status.Error + "\n")
		}
	}
}

func (p *NetemPrompt) StartAll(client proto.NetemClient, cmdArgs []string) {
	ack, err := client.TopologyStartAll(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		RedPrintf("Unable to start all nodes: %v\n", err)
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

func (p *NetemPrompt) StopAll(client proto.NetemClient, cmdArgs []string) {
	ack, err := client.TopologyStopAll(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		RedPrintf("Unable to stop all nodes: %v\n", err)
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
	// first, check editor exists
	if _, err := exec.LookPath(options.ConsoleConfig.Editor); err != nil {
		RedPrintf("Editor set in config, %s, is not found\n", options.ConsoleConfig.Editor)
		return
	}

	response, err := client.ReadNetworkFile(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		RedPrintf("Unable to get network file: %v\n", err)
		return
	}
	// write temp file for edition
	tempFilename := path.Join("/tmp", "gonetem-network-"+p.prjID)
	if err := os.WriteFile(tempFilename, response.GetData(), 0644); err != nil {
		RedPrintf("Unable to write temp file for edition: %v\n", err)
		return
	}
	defer os.Remove(tempFilename)

	if err := EditFile(tempFilename, options.ConsoleConfig.Editor); err != nil {
		RedPrintf("Unable to edit temp file: %v\n", err)
		return
	}

	data, err := os.ReadFile(tempFilename)
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
	stream, err := client.TopologyRun(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		RedPrintf("Unable to run the project: %v\n", err)
		return
	}

	mpBar := mpb.New(mpb.WithWidth(48))
	bars := make([]ProgressBarT, 4)

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			ProgressForceComplete(bars)
			break
		} else if err != nil {
			ProgressAbort(bars, true)
			RedPrintf("Unable to run topology: %v\n", err)
			return
		}

		ProgressHandleMsg(mpBar, bars, msg)
	}

	mpBar.Wait()
}

func (p *NetemPrompt) Reload(client proto.NetemClient, cmdArgs []string) {
	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Prefix = "Reload project " + p.prjPath + " : "
	s.Start()

	stream, err := client.TopologyReload(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		RedPrintf("Unable to reload the project: %v\n", err)
		s.Stop()
		return
	}

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			RedPrintf("Unable to run topology: %v\n", err)
			break
		}

		switch msg.Code {
		case proto.TopologyRunMsg_NODE_MESSAGES:
			s.Stop()
			for _, nMessages := range msg.NodeMessages {
				if len(nMessages.Messages) > 0 {
					fmt.Println(color.YellowString(nMessages.Name + ":"))
					for _, msg := range nMessages.Messages {
						if msg != "" {
							fmt.Println(color.YellowString("  - " + msg))
						}
					}
				}
			}
		}
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
		return fmt.Errorf("unable to connect to server: %v", err)
	} else {
		defer client.Conn.Close()

		_, err = client.Client.CloseProject(context.Background(), &proto.ProjectRequest{Id: p.prjID})
		if err != nil {
			return fmt.Errorf("unable to close project: %v", err)
		}
	}

	return nil
}

func NewNetemPrompt(server, prjID, prjPath string) *NetemPrompt {
	p := &NetemPrompt{
		server, prjID, prjPath,
		make([]*exec.Cmd, 0), make(map[string]*NetemCommand),
		make([]NetemNode, 0),
	}
	p.RegisterCommands()
	p.refreshNodeList()

	return p
}
