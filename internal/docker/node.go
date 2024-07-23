package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/google/shlex"
	"github.com/moby/term"
	"github.com/mroy31/gonetem/internal/link"
	"github.com/mroy31/gonetem/internal/options"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netns"
)

type VrrpOptions struct {
	Interface int
	Group     int
	Address   string
}

type DockerNodeOptions struct {
	Name      string
	ShortName string
	Ipv6      bool
	Mpls      bool
	Vrfs      []string
	Vrrps     []VrrpOptions
	Volumes   []string
}

type DockerNodeStatus struct {
	Running bool
}

type DockerNode struct {
	PrjID          string
	ID             string
	Name           string
	ShortName      string
	Config         *options.DockerNodeConfig
	Interfaces     map[string]link.IfState
	LocalNetnsName string
	Running        bool
	ConfigLoaded   bool
	Mpls           bool
	Vrfs           []string
	Vrrps          []VrrpOptions
	Volumes        []string
	Logger         *logrus.Entry
}

func (n *DockerNode) GetName() string {
	return n.Name
}

func (n *DockerNode) GetShortName() string {
	if n.ShortName == "" {
		return n.Name
	}
	return n.ShortName
}

func (n *DockerNode) GetType() string {
	return "docker"
}

func (n *DockerNode) GetFullType() string {
	return fmt.Sprintf("docker.%s", n.Config.Type)
}

func (n *DockerNode) GetStatus() (DockerNodeStatus, error) {
	client, err := NewDockerClient()
	if err != nil {
		return DockerNodeStatus{}, err
	}
	defer client.Close()

	state, err := client.GetState(context.Background(), n.ID)
	if err != nil {
		return DockerNodeStatus{}, err
	}

	return DockerNodeStatus{
		Running: state == "running",
	}, nil
}

func (n *DockerNode) IsRunning() bool {
	return n.Running
}

func (n *DockerNode) Create(imgName string, ipv6 bool) error {
	client, err := NewDockerClient()
	if err != nil {
		return err
	}
	defer client.Close()

	ctx := context.Background()

	present, err := client.IsImagePresent(ctx, imgName)
	if err != nil {
		return err
	} else if !present {
		return fmt.Errorf("docker image %s not present", imgName)
	}

	containerName := fmt.Sprintf("%s%s.%s", options.NETEM_ID, n.PrjID, n.Name)
	if n.ID, err = client.Create(ctx, imgName, containerName, n.Name, n.Volumes, ipv6, n.Mpls); err != nil {
		return err
	}

	return nil
}

func (n *DockerNode) GetNetns() (netns.NsHandle, error) {
	if n.Running {
		return n.GetRunningNetns()
	}

	return n.GetLocalNetns()
}

func (n *DockerNode) GetRunningNetns() (netns.NsHandle, error) {
	if !n.Running {
		return netns.NsHandle(0), fmt.Errorf("node %s Not running", n.GetName())
	}

	client, err := NewDockerClient()
	if err != nil {
		return netns.NsHandle(0), err
	}
	defer client.Close()

	pid, err := client.Pid(context.Background(), n.ID)
	if err != nil {
		return netns.NsHandle(0), err
	}
	return netns.GetFromPid(pid)
}

func (n *DockerNode) GetLocalNetns() (netns.NsHandle, error) {
	if n.LocalNetnsName == "" {
		// first created local named ns to detach interface without delete it
		n.LocalNetnsName = fmt.Sprintf("%s%s", n.PrjID, n.Name)
		ns, err := link.CreateNetns(n.LocalNetnsName)
		if err != nil {
			n.LocalNetnsName = ""
			return netns.NsHandle(0),
				fmt.Errorf("error when creating node netns '%s': %v", n.LocalNetnsName, err)
		}

		return ns, nil
	}

	localNS, err := netns.GetFromName(n.LocalNetnsName)
	if err != nil {
		return netns.NsHandle(0), fmt.Errorf("unable to get netns associated to node %s: %v", n.Name, err)
	}

	return localNS, nil
}

func (n *DockerNode) GetInterfaceName(ifIndex int) string {
	return fmt.Sprintf("eth%d", ifIndex)
}

func (n *DockerNode) AddInterface(ifName string, ifIndex int, ns netns.NsHandle) error {
	targetIfName := n.GetInterfaceName(ifIndex)
	if err := link.RenameLink(ifName, targetIfName, ns); err != nil {
		return err
	}

	n.Interfaces[targetIfName] = link.IFSTATE_UP
	if n.Running {
		n.PrepareInterface(targetIfName)
	}

	return nil
}

func (n *DockerNode) PrepareInterface(ifName string) {
	client, err := NewDockerClient()
	if err != nil {
		return
	}
	defer client.Close()

	// disable tcp offloading
	cmd := []string{"ethtool", "-K", ifName, "tx", "off"}
	if _, err := client.Exec(context.Background(), n.ID, cmd); err != nil {
		n.Logger.Warnf("Unable to disable tcp offloading on %s", ifName)
	}

	// enable MPLS forwarding
	if n.Mpls {
		cmd = []string{"sysctl", "-w", "net.mpls.conf." + ifName + ".input=1"}
		if _, err := client.Exec(context.Background(), n.ID, cmd); err != nil {
			n.Logger.Warnf("Unable to enable MPLS on %s", ifName)
		}
	}
}

func (n *DockerNode) Capture(ifIndex int, out io.Writer) error {
	if !n.Running {
		return errors.New("not running")
	}

	client, err := NewDockerClient()
	if err != nil {
		return err
	}
	defer client.Close()

	ifName := n.GetInterfaceName(ifIndex)
	cmd := []string{"tcpdump", "-w", "-", "-s", "0", "-U", "-i", ifName}
	return client.ExecOutStream(context.Background(), n.ID, cmd, out)
}

func (n *DockerNode) ExecCommand(
	cmd []string,
	in io.ReadCloser,
	out io.Writer,
	tty bool,
	ttyHeight uint,
	ttyWidth uint,
	resizeCh chan term.Winsize) error {
	if !n.Running {
		return errors.New("not running")
	}

	client, err := NewDockerClient()
	if err != nil {
		return err
	}
	defer client.Close()

	return client.ExecTty(
		context.Background(),
		n.ID,
		cmd, in, out,
		tty, ttyHeight, ttyWidth,
		resizeCh)
}

func (n *DockerNode) GetConsoleCmd(shell bool) ([]string, error) {
	cmd := n.Config.Commands.Console
	if shell {
		cmd = n.Config.Commands.Shell
	}

	cmds, err := shlex.Split(cmd)
	if err != nil {
		return []string{}, fmt.Errorf("unable to parse console cmd: %v", err)
	}

	return cmds, nil
}

func (n *DockerNode) Start() error {
	if !n.Running {
		n.Logger.Debug("Start Node")

		client, err := NewDockerClient()
		if err != nil {
			return err
		}
		defer client.Close()

		ctx := context.Background()
		if err := client.Start(ctx, n.ID); err != nil {
			return err
		}
		n.Running = true

		// Attach existing interfaces
		currentNS, err := n.GetLocalNetns()
		if err != nil {
			return err
		}
		defer currentNS.Close()

		targetNS, err := n.GetRunningNetns()
		if err != nil {
			return err
		}
		defer targetNS.Close()

		if err := link.MoveInterfacesNetns(n.Interfaces, currentNS, targetNS); err != nil {
			return fmt.Errorf("unable to attach interfaces: %v", err)
		}

		// Configure interfaces
		for ifName := range n.Interfaces {
			n.PrepareInterface(ifName)
		}
	}

	return nil
}

func (n *DockerNode) Stop() error {
	if n.Running {
		n.Logger.Debug("Stop Node")

		client, err := NewDockerClient()
		if err != nil {
			return err
		}
		defer client.Close()

		// Detach interfaces
		currentNS, err := n.GetRunningNetns()
		if err != nil {
			return err
		}
		defer currentNS.Close()

		targetNS, err := n.GetLocalNetns()
		if err != nil {
			return err
		}
		defer targetNS.Close()

		if err := link.MoveInterfacesNetns(n.Interfaces, currentNS, targetNS); err != nil {
			return fmt.Errorf("unable to attach interfaces: %v", err)
		}

		ctx := context.Background()
		if err := client.Stop(ctx, n.ID); err != nil {
			return err
		}
		n.Running = false
		n.ConfigLoaded = false
	}

	return nil
}

func (n *DockerNode) LoadConfig(confPath string, timeout int) ([]string, error) {
	var messages []string

	if !n.Running {
		n.Logger.Info("LoadConfig: node not running")
		return messages, nil
	}

	if !n.ConfigLoaded {
		ns, _ := n.GetRunningNetns()
		defer ns.Close()

		// create vrfs
		for idx, vrf := range n.Vrfs {
			if _, err := link.CreateVrf(vrf, ns, 10+idx); err != nil {
				return messages, err
			}
		}

		client, err := NewDockerClient()
		if err != nil {
			return messages, err
		}
		defer client.Close()

		ctx := context.Background()
		if timeout > 0 {
			var cancel context.CancelFunc

			ctx, cancel = context.WithTimeoutCause(
				ctx,
				time.Duration(timeout)*time.Second,
				fmt.Errorf("node %s: loadconfig op timeout", n.Name))
			defer cancel()
		}

		// create vrrps interfaces
		for _, vrrpGroup := range n.Vrrps {
			name := fmt.Sprintf("vrrp-%d", vrrpGroup.Interface)
			if _, err := link.CreateMacVlan(
				name, fmt.Sprintf("eth%d", vrrpGroup.Interface),
				vrrpGroup.Group, ns); err != nil {
				return messages, err
			}
			link.SetInterfaceState(name, ns, link.IFSTATE_UP)

			// set ip address
			if _, err := client.Exec(ctx, n.ID, []string{"ip", "addr", "add", vrrpGroup.Address, "dev", name}); err != nil {
				return messages, err
			}

			// modify kernel settings to disable routes when interface
			// is in linkdown state
			_, err := client.Exec(ctx, n.ID, []string{"sysctl", "-w", fmt.Sprintf("net.ipv4.conf.%s.ignore_routes_with_linkdown=1", name)})
			if err != nil {
				return messages, err
			}
		}

		// Load confifuration files/folders
		if _, err := os.Stat(confPath); err == nil {
			for _, confFileOpts := range n.Config.ConfigurationFiles {
				confFile := path.Join(confPath, fmt.Sprintf("%s.%s", n.Name, confFileOpts.DestSuffix))
				if _, err := os.Stat(confFile); os.IsNotExist(err) {
					continue
				}

				if err := client.CopyTo(ctx, n.ID, confFile, confFileOpts.Source); err != nil {
					return messages, fmt.Errorf("unable to load config file %s:\n\tdest: %s\n\t%w", confFile, confFileOpts.Source, err)
				}
			}

			for _, confFolderOpts := range n.Config.ConfigurationFolders {
				filename := fmt.Sprintf("%s.%s", n.Name, confFolderOpts.DestSuffix)
				confFolderArchive := path.Join(confPath, filename)
				if _, err := os.Stat(confFolderArchive); os.IsNotExist(err) {
					continue
				}

				if err := client.CopyTo(ctx, n.ID, confFolderArchive, filepath.Join("/tmp", filename)); err != nil {
					return messages, fmt.Errorf("unable to copy archile file %s:\n\t%w", filename, err)
				}

				if _, err := client.ExecWithWorkingDir(ctx, n.ID, []string{"tar", "xzf", filepath.Join("/tmp", filename)}, "/"); err != nil {
					return messages, fmt.Errorf("unable to extract archile file %s in the node:\n\t%w", filename, err)
				}
			}
		}

		// Execute load config commands
		for _, loadConfigCmd := range n.Config.Commands.LoadConfig {
			canExec := true
			for _, filepath := range loadConfigCmd.CheckFiles {
				if !client.IsFileExist(context.Background(), n.ID, filepath) {
					canExec = false
				}
			}
			if !canExec {
				continue
			}

			cmd, err := shlex.Split(loadConfigCmd.Command)
			if err != nil {
				return messages, fmt.Errorf("unable to parse loadConfig command %s: %v", loadConfigCmd.Command, err)
			}

			output, err := client.Exec(ctx, n.ID, cmd)
			if err != nil {
				return messages, fmt.Errorf("node %s - unable to exec load config cmd %s - %v", n.Name, loadConfigCmd.Command, err)
			} else if output != "" && n.Config.LogOutput {
				messages = append(messages, output)
			}
		}

		n.ConfigLoaded = true
	}

	return messages, nil
}

func (n *DockerNode) ReadConfigFiles(confDir string, timeout int) (map[string][]byte, error) {
	configFilesData := make(map[string][]byte)

	client, err := NewDockerClient()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	filesDir := confDir
	if n.Running || n.ConfigLoaded {
		// create temp directory for the project
		dir, err := os.MkdirTemp(options.ServerConfig.Workdir, "gonetem-config-node"+n.Name+"-")
		if err != nil {
			return nil, fmt.Errorf("unable to create temp folder to save node config: %w", err)
		}

		if err := n.Save(dir, timeout); err != nil {
			return nil, fmt.Errorf("unable to save node configs in temp folder %s: %w", dir, err)
		}
		filesDir = dir

		defer os.RemoveAll(dir)
	}

	for _, confFileOpts := range n.Config.ConfigurationFiles {
		filepath := path.Join(filesDir, fmt.Sprintf("%s.%s", n.Name, confFileOpts.DestSuffix))
		if _, err := os.Stat(filepath); os.IsNotExist(err) {
			configFilesData[confFileOpts.Label] = []byte{}
			continue
		}

		data, err := os.ReadFile(filepath)
		if err != nil {
			return nil, fmt.Errorf("unable to read config file '%s':\n\t%w", filepath, err)
		}
		configFilesData[confFileOpts.Label] = data
	}

	return configFilesData, nil
}

func (n *DockerNode) Save(dstPath string, timeout int) error {
	if !n.Running || !n.ConfigLoaded {
		n.Logger.Info("Save: node not running")
		return nil
	}

	client, err := NewDockerClient()
	if err != nil {
		return err
	}
	defer client.Close()

	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(
			ctx,
			time.Duration(timeout)*time.Second)
		defer cancel()
	}

	// Execute save config commands
	for _, saveConfigCmd := range n.Config.Commands.SaveConfig {
		canExec := true
		for _, filepath := range saveConfigCmd.CheckFiles {
			if !client.IsFileExist(context.Background(), n.ID, filepath) {
				canExec = false
			}
		}
		if !canExec {
			continue
		}

		cmd, err := shlex.Split(saveConfigCmd.Command)
		if err != nil {
			return fmt.Errorf("unable to parse save config command %s: %v", saveConfigCmd.Command, err)
		}

		if _, err := client.Exec(ctx, n.ID, cmd); err != nil {
			return fmt.Errorf("node %s - unable to exec save config cmd %s - %v", n.Name, saveConfigCmd.Command, err)
		}
	}

	for _, confFileOpts := range n.Config.ConfigurationFiles {
		if !client.IsFileExist(ctx, n.ID, confFileOpts.Source) {
			continue
		}

		confFile := path.Join(dstPath, fmt.Sprintf("%s.%s", n.Name, confFileOpts.DestSuffix))
		if err := client.CopyFrom(ctx, n.ID, confFileOpts.Source, confFile); err != nil {
			return fmt.Errorf("unable to save config file %s:\n\t%w", confFile, err)
		}
	}

	for _, confFolderOpts := range n.Config.ConfigurationFolders {
		if !client.IsFolderExist(ctx, n.ID, confFolderOpts.Source) {
			continue
		}

		filename := fmt.Sprintf("%s.%s", n.Name, confFolderOpts.DestSuffix)
		confFolderArchive := path.Join(dstPath, filename)

		if _, err := client.ExecWithWorkingDir(ctx, n.ID, []string{"tar", "czf", "/tmp/" + filename, confFolderOpts.Source}, "/"); err != nil {
			return err
		}

		if err := client.CopyFrom(ctx, n.ID, "/tmp/"+filename, confFolderArchive); err != nil {
			return fmt.Errorf("node %s - unable to save archive file %s:\n\t%v", n.Name, filename, err)
		}
	}

	return nil
}

func (n *DockerNode) CopyFrom(source, dest string) error {
	client, err := NewDockerClient()
	if err != nil {
		return err
	}
	defer client.Close()

	return client.CopyFrom(context.Background(), n.ID, source, dest)
}

func (n *DockerNode) CopyTo(source, dest string) error {
	client, err := NewDockerClient()
	if err != nil {
		return err
	}
	defer client.Close()

	return client.CopyTo(context.Background(), n.ID, source, dest)
}

func (n *DockerNode) GetInterfacesState() map[string]link.IfState {
	return n.Interfaces
}

func (n *DockerNode) SetInterfaceState(ifIndex int, state link.IfState) error {
	for ifName, st := range n.Interfaces {
		if ifName == n.GetInterfaceName(ifIndex) {
			if state != st {
				ns, err := n.GetNetns()
				if err != nil {
					return err
				}
				defer ns.Close()

				if err := link.SetInterfaceState(n.GetInterfaceName(ifIndex), ns, state); err != nil {
					return err
				}
				n.Interfaces[ifName] = state
				return nil
			}
			return nil
		}
	}

	return fmt.Errorf("interface %s.%d not found", n.GetName(), ifIndex)
}

func (n *DockerNode) Close() error {
	if n.ID != "" {
		n.Logger.Debug("Close node")

		client, err := NewDockerClient()
		if err != nil {
			return err
		}
		defer client.Close()

		ctx := context.Background()
		if n.Running {
			client.Stop(ctx, n.ID)
			n.Running = false
		}

		if err := client.Rm(ctx, n.ID); err != nil {
			return err
		}

		// clean attributes
		n.ConfigLoaded = false
		n.Interfaces = make(map[string]link.IfState)
		if n.LocalNetnsName != "" {
			link.DeleteNetns(n.LocalNetnsName)
			n.LocalNetnsName = ""
		}
	}

	return nil
}

func getDockerConfigFromType(nType string) (*options.DockerNodeConfig, error) {
	switch nType {
	case "router":
		return &options.ServerConfig.Docker.Nodes.Router, nil
	case "host":
		return &options.ServerConfig.Docker.Nodes.Host, nil
	case "server":
		return &options.ServerConfig.Docker.Nodes.Server, nil
	default:
		for _, nConfig := range options.ServerConfig.Docker.ExtraNodes {
			if nConfig.Type == nType {
				return &nConfig, nil
			}
		}
		return nil, fmt.Errorf("docker node of type %s does not exist in the configuraiton", nType)
	}
}

func NewDockerNode(prjID string, nType string, dockerOpts DockerNodeOptions) (*DockerNode, error) {
	nConfig, err := getDockerConfigFromType(nType)
	if err != nil {
		return nil, err
	}

	node := &DockerNode{
		PrjID:      prjID,
		ID:         "",
		Name:       dockerOpts.Name,
		ShortName:  dockerOpts.ShortName,
		Config:     nConfig,
		Mpls:       dockerOpts.Mpls,
		Vrfs:       dockerOpts.Vrfs,
		Vrrps:      dockerOpts.Vrrps,
		Volumes:    dockerOpts.Volumes,
		Interfaces: make(map[string]link.IfState),
		Logger: logrus.WithFields(logrus.Fields{
			"project": prjID,
			"node":    dockerOpts.Name,
		}),
	}

	imgName := options.GetDockerImageId(nConfig.Image)
	if err := node.Create(imgName, dockerOpts.Ipv6); err != nil {
		return node, err
	}
	return node, nil
}
