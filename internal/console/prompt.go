package console

import (
	"bytes"
	"context"
	"errors"
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
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/docker/docker/pkg/system"
	"github.com/fatih/color"
	"github.com/google/shlex"
	"github.com/mroy31/gonetem/internal/options"
	"github.com/mroy31/gonetem/internal/proto"
	"github.com/mroy31/gonetem/internal/utils"
	"github.com/vbauerster/mpb/v8"
)

type copyDirection int

const (
	fromNode copyDirection = 1 << iota
	toNode
	acrossNodes = fromNode | toNode
)

var (
	RedPrintf = color.New(color.FgRed).PrintfFunc()
)

func Fatal(msg string, a ...interface{}) {
	RedPrintf(msg+"\n", a...)
	os.Exit(1)
}

// ackError converts the acknowledgement of a server command into an error:
// transport errors are wrapped with the action, errors reported by the
// server itself are passed through as-is.
func ackError(action string, ack *proto.AckResponse, err error) error {
	if err != nil {
		return fmt.Errorf("Unable to %s: %v", action, err)
	}
	if ack.GetStatus().GetCode() == proto.StatusCode_ERROR {
		return errors.New(ack.GetStatus().GetError())
	}
	return nil
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
	Run   func(p *NetemPrompt, cmdArgs []string) error
}

type NetemPrompt struct {
	server        string
	prjID         string
	prjPath       string
	processes     []*exec.Cmd
	commands      map[string]*NetemCommand
	program       *tea.Program
	done          bool
	pendingPrompt *inputPromptRequest
	pendingExec   *pendingEditorRequest
	pendingOutput []string
	pendingClear  bool
	// nodes is used by completion, and refreshed in background by the
	// asynchronous run command: it must stay locked
	nodesMu sync.Mutex
	nodes   []NetemNode
}

type inputPromptRequest struct {
	prefix      string
	completions []Completion
	submit      func(string)
}

// pendingEditorRequest asks the TUI to run an external editor with the
// terminal taken back from bubbletea. finish is called once the editor has
// exited, with the editor error if any, and must consume the edited file.
type pendingEditorRequest struct {
	cmd    *exec.Cmd
	finish func(editorErr error) error
}

// clientExec adapts a command needing a server connection into a prompt
// command, opening the client around its execution.
func clientExec(execFunc func(client proto.NetemClient, cmdArgs []string) error) func(p *NetemPrompt, cmdArgs []string) error {
	return func(p *NetemPrompt, cmdArgs []string) error {
		return p.execWithClient(cmdArgs, execFunc)
	}
}

// clientExecRefreshed is a client command after which the node list is
// refreshed, as the topology changed (run, reload).
func clientExecRefreshed(execFunc func(client proto.NetemClient, cmdArgs []string) error) func(p *NetemPrompt, cmdArgs []string) error {
	return func(p *NetemPrompt, cmdArgs []string) error {
		err := p.execWithClient(cmdArgs, execFunc)
		p.refreshNodeList()
		return err
	}
}

// clientExecNodeOrAll dispatches a node command to its all-nodes variant
// when the node name is "all" (start, stop).
func clientExecNodeOrAll(nodeFunc, allFunc func(client proto.NetemClient, cmdArgs []string) error) func(p *NetemPrompt, cmdArgs []string) error {
	return func(p *NetemPrompt, cmdArgs []string) error {
		if cmdArgs[0] == "all" {
			return p.execWithClient(cmdArgs, allFunc)
		}
		return p.execWithClient(cmdArgs, nodeFunc)
	}
}

// consoleExec opens one node console, or one per running node with "all".
func (p *NetemPrompt) consoleExec(client proto.NetemClient, cmdArgs []string, shell bool) error {
	if cmdArgs[0] == "all" {
		return p.startConsoleAll(client, shell)
	}
	return p.startConsole(client, cmdArgs[0], shell)
}

func (p *NetemPrompt) RegisterCommands() {
	p.commands["capture"] = &NetemCommand{
		Desc:  "Capture trafic on an interface",
		Usage: "capture <node_name>.<if_number>",
		Args:  []string{`^\w+\.\d+$`},
		Run:   (*NetemPrompt).Capture,
	}
	p.commands["check"] = &NetemCommand{
		Desc:  "Check that the topology file is correct. If not, return found errors",
		Usage: "check",
		Args:  []string{},
		Run:   clientExec(p.Check),
	}
	p.commands["clear"] = &NetemCommand{
		Desc:  "Clear the command and output history displayed above the prompt",
		Usage: "clear",
		Args:  []string{},
		Run:   (*NetemPrompt).Clear,
	}
	p.commands["config"] = &NetemCommand{
		Desc:  "Save the configuration files in the specified folder",
		Usage: "config <dest_path>",
		Args:  []string{`^.+$`},
		Run:   clientExec(p.Config),
	}
	p.commands["console"] = &NetemCommand{
		Desc:  "Open a console for a node",
		Usage: "console <node_name>",
		Args:  []string{`^\w+$`},
		Run: clientExec(func(client proto.NetemClient, cmdArgs []string) error {
			return p.consoleExec(client, cmdArgs, false)
		}),
	}
	p.commands["copy"] = &NetemCommand{
		Desc:  "Copy a file from/to a node",
		Usage: "copy sourceFile <node>:destFile",
		Args:  []string{`^.+$`, `^.+$`},
		Run: func(p *NetemPrompt, cmdArgs []string) error {
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
				return p.CopyFrom(srcNode, srcPath, destPath)
			case toNode:
				return p.CopyTo(srcPath, destNode, destPath)
			case acrossNodes:
				return errors.New("copying between containers is not supported")
			default:
				return errors.New("must specify at least one container source")
			}
		},
	}
	p.commands["edit"] = &NetemCommand{
		Desc:  "Edit the topology",
		Usage: "edit",
		Args:  []string{},
		Run:   clientExec(p.Edit),
	}
	p.commands["exec"] = &NetemCommand{
		Desc:  "Exec a command on a node",
		Usage: "exec <node_name> <cmd>",
		Args:  []string{`^\w+$`, `^.+$`},
		Run:   clientExec(p.Exec),
	}
	p.commands["ifState"] = &NetemCommand{
		Desc:  "Enable/disable a node interface",
		Usage: "ifState <node_name>.<if_number> up|down",
		Args:  []string{`^\w+\.\d+$`, `^up|down$`},
		Run:   clientExec(p.IfState),
	}
	p.commands["reload"] = &NetemCommand{
		Desc:  "Reload the project",
		Usage: "reload",
		Args:  []string{},
		Run:   clientExecRefreshed(p.Reload),
	}
	p.commands["restart"] = &NetemCommand{
		Desc:  "Restart a node",
		Usage: "restart <node_name>",
		Args:  []string{`^\w+$`},
		Run:   clientExec(p.Restart),
	}
	p.commands["run"] = &NetemCommand{
		Desc:  "Start the project",
		Usage: "run",
		Args:  []string{},
		Run:   (*NetemPrompt).Run,
	}
	p.commands["save"] = &NetemCommand{
		Desc:  "Save the project",
		Usage: "save",
		Args:  []string{},
		Run:   (*NetemPrompt).Save,
	}
	p.commands["saveAs"] = &NetemCommand{
		Desc:  "Save the project in a new file",
		Usage: "saveAs <project_path>/<name>.gnet",
		Args:  []string{`^.*\.gnet$`},
		Run:   (*NetemPrompt).SaveAs,
	}
	p.commands["shell"] = &NetemCommand{
		Desc:  "Open a shell console for a node",
		Usage: "shell <node_name>",
		Args:  []string{`^\w+$`},
		Run: clientExec(func(client proto.NetemClient, cmdArgs []string) error {
			return p.consoleExec(client, cmdArgs, true)
		}),
	}
	p.commands["start"] = &NetemCommand{
		Desc:  "Start a node",
		Usage: "start <node_name>",
		Args:  []string{`^\w+$`},
		Run:   clientExecNodeOrAll(p.Start, p.StartAll),
	}
	p.commands["stop"] = &NetemCommand{
		Desc:  "Stop a node",
		Usage: "stop <node_name>",
		Args:  []string{`^\w+$`},
		Run:   clientExecNodeOrAll(p.Stop, p.StopAll),
	}
	p.commands["status"] = &NetemCommand{
		Desc:  "Display the state of the project",
		Usage: "status",
		Args:  []string{},
		Run:   clientExec(p.Status),
	}
	p.commands["viewConfig"] = &NetemCommand{
		Desc:  "Display the current configuration of a node",
		Usage: "viewConfig <node_name>",
		Args:  []string{`^\w+$`},
		Run:   clientExec(p.ViewNodeConfiguration),
	}
}

func (p *NetemPrompt) Execute(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	if s == "quit" || s == "exit" {
		p.done = true
		return nil
	}

	if s == "help" {
		var output strings.Builder
		for name, cmd := range p.commands {
			fmt.Fprintln(&output, color.BlueString(name))
			fmt.Fprintln(&output, "  "+cmd.Desc)
			fmt.Fprintln(&output, "  Usage: "+cmd.Usage)
		}
		p.queuePromptOutput(output.String())
		return nil
	}

	args, err := shlex.Split(s)
	if err != nil {
		return fmt.Errorf("Bad command line: %v", err)
	}

	cmd, found := p.commands[args[0]]
	if !found {
		return errors.New("Unknown command, enter help for details")
	}

	// check args
	if len(args) != len(cmd.Args)+1 {
		return fmt.Errorf("Wrong number of arguments for '%s'\n\tusage: %s", args[0], cmd.Usage)
	}
	for idx, argRe := range cmd.Args {
		r, _ := regexp.Compile(argRe)
		if !r.MatchString(args[idx+1]) {
			return fmt.Errorf("Wrong format for argument %d\n\tusage: %s", idx+1, cmd.Usage)
		}
	}

	// run the command
	return cmd.Run(p, args[1:])
}

func (p *NetemPrompt) refreshNodeList() {
	client, err := NewClient(p.server)
	if err != nil {
		return
	}
	defer client.Conn.Close()

	status, err := client.Client.ProjectGetStatus(
		context.Background(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		return
	}

	nodes := make([]NetemNode, 0)
	for _, node := range status.GetNodes() {
		nStatus := NetemNode{
			Name:       node.GetName(),
			Interfaces: make([]string, 0),
		}
		for _, ifStatus := range node.GetInterfaces() {
			nStatus.Interfaces = append(nStatus.Interfaces, ifStatus.Name)
		}

		nodes = append(nodes, nStatus)
	}

	// the list may be replaced by the asynchronous run command while
	// completions read it from the TUI thread
	p.nodesMu.Lock()
	p.nodes = nodes
	p.nodesMu.Unlock()
}

// nodeList returns the current node list for completion.
func (p *NetemPrompt) nodeList() []NetemNode {
	p.nodesMu.Lock()
	defer p.nodesMu.Unlock()
	return p.nodes
}

func (p *NetemPrompt) execWithClient(cmdArgs []string, execFunc func(client proto.NetemClient, cmdArgs []string) error) error {
	client, err := NewClient(p.server)
	if err != nil {
		return fmt.Errorf("Unable to connect to gonetem server: %v", err)
	}
	defer client.Conn.Close()

	return execFunc(client.Client, cmdArgs)
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

func (p *NetemPrompt) CopyFrom(srcNode, srcPath, destPath string) error {
	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("Unable to create/open %s: %v", destPath, err)
	}
	defer file.Close()

	client, err := NewClient(p.server)
	if err != nil {
		return fmt.Errorf("Unable to connect to gonetem server: %v", err)
	}
	defer client.Conn.Close()

	stream, err := client.Client.NodeCopyFrom(context.Background(), &proto.CopyMsg{
		Code:     proto.CopyMsg_INIT,
		PrjId:    p.prjID,
		Node:     srcNode,
		NodePath: srcPath,
	})
	if err != nil {
		return fmt.Errorf("CopyFrom %s:%s returns an error: %v", srcNode, srcPath, err)
	}

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		switch msg.GetCode() {
		case proto.CopyMsg_DATA:
			file.Write(msg.GetData())
		case proto.CopyMsg_ERROR:
			return fmt.Errorf("CopyFrom %s:%s returns an error: %s", srcNode, srcPath, string(msg.GetData()))
		}
	}
	return nil
}

func (p *NetemPrompt) CopyTo(srcPath, destNode, destPath string) error {
	stat, err := os.Stat(srcPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("File %s does not exist", srcPath)
	} else if err != nil {
		return fmt.Errorf("Unable to stat %s: %v", srcPath, err)
	}

	// check it is a regular file
	if !stat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", srcPath)
	}

	client, err := NewClient(p.server)
	if err != nil {
		return fmt.Errorf("Unable to connect to gonetem server: %v", err)
	}
	defer client.Conn.Close()

	buffer := make([]byte, 1024)
	file, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("Unable to open %s: %v", srcPath, err)
	}
	defer file.Close()

	stream, err := client.Client.NodeCopyTo(context.Background())
	if err != nil {
		return fmt.Errorf("CopyTo: %v", err)
	}

	if err := stream.Send(&proto.CopyMsg{
		Code:     proto.CopyMsg_INIT,
		PrjId:    p.prjID,
		Node:     destNode,
		NodePath: destPath,
	}); err != nil {
		return fmt.Errorf("Unable to init CopyTo: %v", err)
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
		return fmt.Errorf("Error in CopyTo cmd: %v", err)
	} else if ack.GetStatus().GetCode() == proto.StatusCode_ERROR {
		return fmt.Errorf("CopyTo returns an error: %s", string(ack.Status.Error))
	}
	return nil
}

func (p *NetemPrompt) Capture(cmdArgs []string) error {
	args := strings.Split(cmdArgs[0], ".")
	ifIndex, _ := strconv.Atoi(args[1])

	// Check wireshark is present
	wiresharkPath, err := exec.LookPath("wireshark")
	if err != nil {
		return errors.New("wireshark is not installed")
	}

	client, err := NewClient(p.server)
	if err != nil {
		return fmt.Errorf("Unable to connect to gonetem server: %v", err)
	}

	stream, err := client.Client.NodeCapture(context.Background(), &proto.NodeInterfaceRequest{
		PrjId:   p.prjID,
		Node:    args[0],
		IfIndex: int32(ifIndex),
	})
	if err != nil {
		client.Conn.Close()
		return fmt.Errorf("Error when start capturing %v", err)
	}

	msg, err := stream.Recv()
	if err != nil {
		client.Conn.Close()
		return fmt.Errorf("Error when start capturing %v", err)
	}

	if msg.GetCode() == proto.CaptureSrvMsg_ERROR {
		client.Conn.Close()
		return fmt.Errorf("%s", string(msg.GetData()))
	}

	rIn, wIn := io.Pipe()

	captureArgs := []string{
		"-o", fmt.Sprintf("gui.window_title:%s@%s", args[1], args[0]),
		"-k", "-i", "-"}
	cmd := exec.Command(wiresharkPath, captureArgs...)
	cmd.Stdin = rIn

	if err := cmd.Start(); err != nil {
		client.Conn.Close()
		return fmt.Errorf("Error when starting wireshark: %v", err)
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
				p.sendPromptError(fmt.Sprintf("Error when capturing, stop wireshark: %s", string(msg.GetData())))
				cmd.Process.Signal(os.Kill)
				return
			}
		}

	}()

	return nil
}

func (p *NetemPrompt) sendPromptError(message string) {
	p.program.Send(promptErrorMsg(message))
}

func (p *NetemPrompt) IfState(client proto.NetemClient, cmdArgs []string) error {
	state, found := map[string]proto.IfState{
		"up":   proto.IfState_UP,
		"down": proto.IfState_DOWN,
	}[cmdArgs[1]]
	if !found {
		return errors.New("State is not valid: up|down expected")
	}

	ifArgs := strings.Split(cmdArgs[0], ".")
	ifIndex, _ := strconv.Atoi(ifArgs[1])

	_, err := client.NodeSetIfState(
		context.Background(),
		&proto.NodeIfStateRequest{
			PrjId:   p.prjID,
			Node:    ifArgs[0],
			IfIndex: int32(ifIndex),
			State:   state,
		})
	if err != nil {
		return fmt.Errorf("Unable to change interface state: %v", err)
	}
	return nil
}

func (p *NetemPrompt) Check(client proto.NetemClient, cmdArgs []string) error {
	ack, err := client.TopologyCheck(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		return err
	}
	if err := ackError("check topology", ack, nil); err != nil {
		return err
	}
	p.queuePromptOutput(color.GreenString("Network is OK\n"))
	return nil
}

func (p *NetemPrompt) Config(client proto.NetemClient, cmdArgs []string) error {
	dstPath := cmdArgs[0]
	stat, err := os.Stat(dstPath)
	if err != nil {
		return fmt.Errorf("Unable to get stat on dest path '%s'\n\t%v", dstPath, err)
	} else if !stat.IsDir() {
		return fmt.Errorf("Dest path '%s' is not a directory", dstPath)
	}

	response, err := client.ProjectGetNodeConfigs(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		return fmt.Errorf("Unable to get project configuration files: %v", err)
	}

	buffer := bytes.NewBuffer(response.GetData())
	if err := utils.OpenArchive(dstPath, buffer); err != nil {
		return fmt.Errorf("Unable to extract configuration files: %v", err)
	}
	return nil
}

func (p *NetemPrompt) Exec(client proto.NetemClient, cmdArgs []string) error {
	node := cmdArgs[0]

	cmd, err := shlex.Split(cmdArgs[1])
	if err != nil {
		return err
	}

	return nodeExec(client, p.prjID, node, cmd)
}

func (p *NetemPrompt) startConsole(client proto.NetemClient, nodeName string, shell bool) error {
	response, err := client.ProjectGetStatus(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		return fmt.Errorf("Unable to get project status: %v", err)
	}

	// check project and node status
	if !response.Running {
		return errors.New("Project is not running")
	}
	for _, n := range response.GetNodes() {
		if n.Name == nodeName && !n.Running {
			return errors.New("Node is not running")
		}
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
		return fmt.Errorf("Unable to parse terminal line in config file: %v", err)
	}
	err = tmpl.Execute(&buf, consoleCmd)
	if err != nil {
		return fmt.Errorf("Unable to parse terminal line in config file: %v", err)
	}

	args, err := shlex.Split(buf.String())
	if err != nil {
		return fmt.Errorf("Bad command line: %v", err)
	}

	// search term command
	termPath, err := exec.LookPath(args[0])
	if err != nil {
		return fmt.Errorf("terminal '%s' is not installed", args[0])
	}

	cmd := exec.Command(termPath, args[1:]...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Error when starting console: %v", err)
	}

	p.processes = append(p.processes, cmd)
	return nil
}

func (p *NetemPrompt) startConsoleAll(client proto.NetemClient, shell bool) error {
	response, err := client.ProjectGetStatus(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		return fmt.Errorf("Unable to get project status: %v", err)
	}

	for _, node := range response.GetNodes() {
		if node.Running {
			if err := p.startConsole(client, node.GetName(), shell); err != nil {
				return err
			}
		}
	}
	return nil
}

// save starts the project save in background and reports progress to the
// prompt model: bubbletea owns the screen and would overwrite the bars
// rendered by mpb. As the client of the command is closed as soon as the
// command returns, it opens its own connection kept alive until the save
// stream is fully consumed.
func (p *NetemPrompt) save(dstPath string) error {
	client, err := NewClient(p.server)
	if err != nil {
		return fmt.Errorf("Unable to connect to gonetem server: %v", err)
	}

	stream, err := client.Client.ProjectSave(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		client.Conn.Close()
		return fmt.Errorf("Unable to save project: %v", err)
	}

	go func() {
		defer client.Conn.Close()
		defer p.program.Send(promptProgressMsg(""))

		p.program.Send(promptProgressMsg(progressText("Save nodes:", 0, 0)))

		total := 0
		saved := 0
		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				p.program.Send(promptOutputMsg(color.GreenString("Project saved to " + dstPath + "\n")))
				return
			} else if err != nil {
				p.sendPromptError(fmt.Sprintf("Unable to save project: %v", err))
				return
			}

			switch msg.Code {
			case proto.ProjectSaveMsg_NODE_COUNT:
				total = int(msg.GetTotal())
				p.program.Send(promptProgressMsg(progressText("Save nodes:", 0, total)))

			case proto.ProjectSaveMsg_NODE_SAVE:
				saved++
				p.program.Send(promptProgressMsg(progressText("Save nodes:", saved, total)))

			case proto.ProjectSaveMsg_DATA:
				if err := os.WriteFile(dstPath, msg.GetData(), 0644); err != nil {
					p.sendPromptError(fmt.Sprintf("Unable to write saved project to %s: %v", dstPath, err))
					return
				}
			}
		}
	}()

	return nil
}

func (p *NetemPrompt) Save(cmdArgs []string) error {
	if p.prjPath == "" {
		return errors.New("Project path is empty, use saveAs command if you connect to running project")
	}

	return p.save(p.prjPath)
}

func (p *NetemPrompt) SaveAs(cmdArgs []string) error {
	return p.save(cmdArgs[0])
}

func (p *NetemPrompt) Start(client proto.NetemClient, cmdArgs []string) error {
	ack, err := client.NodeStart(p.getCancelContext(), &proto.NodeRequest{PrjId: p.prjID, Node: cmdArgs[0]})
	return ackError("start node", ack, err)
}

func (p *NetemPrompt) StartAll(client proto.NetemClient, cmdArgs []string) error {
	ack, err := client.TopologyStartAll(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	return ackError("start all nodes", ack, err)
}

func (p *NetemPrompt) Stop(client proto.NetemClient, cmdArgs []string) error {
	ack, err := client.NodeStop(context.Background(), &proto.NodeRequest{PrjId: p.prjID, Node: cmdArgs[0]})
	return ackError("stop node", ack, err)
}

func (p *NetemPrompt) StopAll(client proto.NetemClient, cmdArgs []string) error {
	ack, err := client.TopologyStopAll(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	return ackError("stop all nodes", ack, err)
}

func (p *NetemPrompt) Restart(client proto.NetemClient, cmdArgs []string) error {
	ack, err := client.NodeRestart(context.Background(), &proto.NodeRequest{PrjId: p.prjID, Node: cmdArgs[0]})
	return ackError("restart node", ack, err)
}

func (p *NetemPrompt) Status(client proto.NetemClient, cmdArgs []string) error {
	response, err := client.ProjectGetStatus(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		return fmt.Errorf("Unable to get project status: %v", err)
	}

	var output strings.Builder
	fmt.Fprintln(&output, "Project "+response.GetName())
	fmt.Fprintln(&output, "- Id: "+response.GetId())
	fmt.Fprintln(&output, "- OpenAt: "+response.GetOpenAt())
	fmt.Fprint(&output, "- State: ")
	if response.GetRunning() {
		fmt.Fprintln(&output, color.GreenString("Running"))
	} else {
		fmt.Fprintln(&output, color.YellowString("Not Running"))
	}

	if len(response.GetNodes()) > 0 {
		fmt.Fprintln(&output, "- Nodes:")
		for _, nodeInfo := range response.GetNodes() {
			fmt.Fprint(&output, "   - "+nodeInfo.GetName()+": ")
			if nodeInfo.GetRunning() {
				fmt.Fprintln(&output, color.GreenString("Running"))
				for _, i := range nodeInfo.GetInterfaces() {
					fmt.Fprint(&output, "     - "+i.GetName()+": ")
					switch i.GetState() {
					case proto.IfState_DOWN:
						fmt.Fprintln(&output, color.MagentaString("Down"))
					case proto.IfState_UP:
						fmt.Fprintln(&output, color.GreenString("Up"))
					}
				}
			} else {
				fmt.Fprintln(&output, color.MagentaString("Not Running"))
			}
		}
	}
	p.queuePromptOutput(output.String())
	return nil
}

// queuePromptOutput accumulates the command output to display in the TUI
// transcript, drained by the prompt model once the command returns.
func (p *NetemPrompt) queuePromptOutput(output string) {
	p.pendingOutput = append(p.pendingOutput, output)
}

func (p *NetemPrompt) takePromptOutput() []string {
	output := p.pendingOutput
	p.pendingOutput = nil
	return output
}

func (p *NetemPrompt) takePendingPrompt() *inputPromptRequest {
	request := p.pendingPrompt
	p.pendingPrompt = nil
	return request
}

func (p *NetemPrompt) takePendingExec() *pendingEditorRequest {
	request := p.pendingExec
	p.pendingExec = nil
	return request
}

func (p *NetemPrompt) takePendingClear() bool {
	pending := p.pendingClear
	p.pendingClear = false
	return pending
}

// Clear asks the TUI to empty the transcript displayed above the prompt.
func (p *NetemPrompt) Clear(cmdArgs []string) error {
	p.pendingClear = true
	return nil
}

func (p *NetemPrompt) Edit(client proto.NetemClient, cmdArgs []string) error {
	// first, check editor exists
	if _, err := exec.LookPath(options.ConsoleConfig.Editor); err != nil {
		return fmt.Errorf("Editor set in config, %s, is not found", options.ConsoleConfig.Editor)
	}

	response, err := client.ReadNetworkFile(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		return fmt.Errorf("Unable to get network file: %v", err)
	}
	// write temp file for edition
	if err := os.WriteFile(p.tempNetworkFilename(), response.GetData(), 0644); err != nil {
		return fmt.Errorf("Unable to write temp file for edition: %v", err)
	}

	// bubbletea owns the terminal (raw mode and alt screen): the editor must
	// not run inside the event loop. Queue the editor command so the program
	// can run it with the terminal given back, then upload through finish.
	p.pendingExec = &pendingEditorRequest{
		cmd:    exec.Command(options.ConsoleConfig.Editor, p.tempNetworkFilename()),
		finish: p.uploadEditedNetwork,
	}
	return nil
}

// uploadEditedNetwork reads back the edited topology file and sends it to
// the server. It runs after the external editor has exited, with its own
// client connection as the one of the command is already closed.
func (p *NetemPrompt) uploadEditedNetwork(editorErr error) error {
	tempFilename := p.tempNetworkFilename()
	defer os.Remove(tempFilename)

	if editorErr != nil {
		return fmt.Errorf("Unable to edit temp file: %v", editorErr)
	}

	data, err := os.ReadFile(tempFilename)
	if err != nil {
		return fmt.Errorf("Unable to read edited network file: %v", err)
	}

	client, err := NewClient(p.server)
	if err != nil {
		return fmt.Errorf("Unable to connect to gonetem server: %v", err)
	}
	defer client.Conn.Close()

	if _, err := client.Client.WriteNetworkFile(context.Background(), &proto.WNetworkRequest{
		Id:   p.prjID,
		Data: data,
	}); err != nil {
		return fmt.Errorf("Unable to write modified network file on server: %v", err)
	}

	p.queuePromptOutput(color.GreenString("Network file has been updated, reload the project to take it in account\n"))
	return nil
}

func (p *NetemPrompt) tempNetworkFilename() string {
	return path.Join("/tmp", "gonetem-network-"+p.prjID)
}

// Run starts the topology in background and reports progress to the prompt
// model: bubbletea owns the screen and would overwrite the bars rendered by
// mpb. As the client of the command is closed as soon as the command
// returns, it opens its own connection kept alive until the run stream is
// fully consumed.
func (p *NetemPrompt) Run(cmdArgs []string) error {
	client, err := NewClient(p.server)
	if err != nil {
		return fmt.Errorf("Unable to connect to gonetem server: %v", err)
	}

	stream, err := client.Client.TopologyRun(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		client.Conn.Close()
		return fmt.Errorf("Unable to run the project: %v", err)
	}

	go func() {
		defer client.Conn.Close()
		defer p.program.Send(promptProgressMsg(""))
		// node list needs to be updated for completion
		defer p.refreshNodeList()

		progress := newRunProgress()
		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				return
			} else if err != nil {
				p.sendPromptError(fmt.Sprintf("Unable to run topology: %v", err))
				return
			}

			if msg.GetCode() == proto.TopologyRunMsg_NODE_MESSAGES {
				p.program.Send(promptOutputMsg(nodeMessagesOutput(msg)))
			}
			p.program.Send(promptProgressMsg(progress.handle(msg)))
		}
	}()

	return nil
}

func (p *NetemPrompt) ViewNodeConfiguration(client proto.NetemClient, cmdArgs []string) error {
	response, err := client.NodeReadConfigFiles(context.Background(), &proto.NodeRequest{PrjId: p.prjID, Node: cmdArgs[0]})
	if err != nil {
		return fmt.Errorf("Unable to to read configuration files: %v", err)
	}

	if len(response.Files) == 1 {
		p.displayConfiguration(response.Files[0])
		return nil
	}

	p.pendingPrompt = &inputPromptRequest{
		prefix:      "Select configuration file: ",
		completions: ConfigurationCompletions(response),
		submit: func(configName string) {
			for _, configFile := range response.Files {
				if configFile.Name == configName {
					p.displayConfiguration(configFile)
					return
				}
			}
			p.queuePromptOutput(color.YellowString("Configuration file %s not found\n", configName))
		},
	}
	return nil
}

func (p *NetemPrompt) displayConfiguration(configFile *proto.ConfigFilesResponse_ConfigFile) {
	p.queuePromptOutput("#####  " + configFile.Name + "  #####\n")
	p.queuePromptOutput(fmt.Sprintf("\n%s\n", configFile.Data))
}

func (p *NetemPrompt) Reload(client proto.NetemClient, cmdArgs []string) error {
	stream, err := client.TopologyReload(context.Background(), &proto.ProjectRequest{Id: p.prjID})
	if err != nil {
		return fmt.Errorf("Unable to reload the project: %v", err)
	}

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("Unable to run topology: %v", err)
		}

		if msg.Code == proto.TopologyRunMsg_NODE_MESSAGES {
			p.queuePromptOutput(nodeMessagesOutput(msg))
		}
	}
	return nil
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

		stream, err := client.Client.ProjectClose(context.Background(), &proto.ProjectRequest{Id: p.prjID})
		if err != nil {
			return fmt.Errorf("unable to close project: %v", err)
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
				return err
			}

			ProgressCloseHandleMsg(mpBar, bars, msg)
		}

		mpBar.Wait()
	}

	return nil
}

func NewNetemPrompt(server, prjID, prjPath string) *NetemPrompt {
	p := &NetemPrompt{
		server:    server,
		prjID:     prjID,
		prjPath:   prjPath,
		processes: make([]*exec.Cmd, 0),
		commands:  make(map[string]*NetemCommand),
		nodes:     make([]NetemNode, 0),
	}
	p.RegisterCommands()
	p.refreshNodeList()

	return p
}
