package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/moby/term"
	"github.com/mroy31/gonetem/internal/link"
	"github.com/mroy31/gonetem/internal/options"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netns"
)

const (
	initScript = "/gonetem-init.sh"
)

type VrrpOptions struct {
	Interface int
	Group     int
	Address   string
}

type DockerNodeOptions struct {
	Name      string
	ShortName string
	Type      string
	ImgName   string
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
	Type           string
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

func (n *DockerNode) CanRunConsole() error {
	if !n.Running {
		return errors.New("not running")
	}
	return nil
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

func (n *DockerNode) Console(shell bool, in io.ReadCloser, out io.Writer, resizeCh chan term.Winsize) error {
	if !n.Running {
		return errors.New("not running")
	}

	client, err := NewDockerClient()
	if err != nil {
		return err
	}
	defer client.Close()

	cmd := []string{"/bin/bash"}
	if !shell {
		switch n.Type {
		case "router":
			cmd = []string{"/usr/bin/vtysh"}
		}
	}

	return client.ExecTty(context.Background(), n.ID, cmd, in, out, resizeCh)
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

		if _, err := os.Stat(confPath); err == nil {
			configFiles := make(map[string]string)
			configFolders := make(map[string]string)

			switch n.Type {
			case "router":
				configFiles[n.Name+".frr.conf"] = "/etc/frr/frr.conf"
			case "host":
				configFiles[n.Name+".net.conf"] = "/tmp/custom.net.conf"
				configFiles[n.Name+".ntp.conf"] = "/etc/ntp.conf"
			case "server":
				configFiles[n.Name+".net.conf"] = "/tmp/custom.net.conf"
				configFiles[n.Name+".ntp.conf"] = "/etc/ntp.conf"
				configFiles[n.Name+".dhcpd.conf"] = "/etc/dhcp/dhcpd.conf"
				configFiles[n.Name+".tftpd-hpa.default"] = "/etc/default/tftpd-hpa"
				configFiles[n.Name+".isc-relay.default"] = "/etc/default/isc-dhcp-relay"
				configFiles[n.Name+".bind.default"] = "/etc/default/named"

				configFolders[n.Name+".tftp-data.tgz"] = "/srv/tftp"
				configFolders[n.Name+".bind-etc.tgz"] = "/etc/bind"
			}

			configFiles[n.Name+".init.conf"] = initScript
			for source, dest := range configFiles {
				source = path.Join(confPath, source)
				if _, err := os.Stat(source); os.IsNotExist(err) {
					continue
				}

				if err := client.CopyTo(ctx, n.ID, source, dest); err != nil {
					return messages, fmt.Errorf("unable to load config file %s:\n\t%w", source, err)
				}
			}

			for source := range configFolders {
				sourcePath := path.Join(confPath, source)
				if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
					continue
				}

				if err := client.CopyTo(ctx, n.ID, sourcePath, filepath.Join("/tmp", source)); err != nil {
					return messages, fmt.Errorf("unable to copy archile file %s:\n\t%w", source, err)
				}

				if _, err := client.ExecWithWorkingDir(ctx, n.ID, []string{"tar", "xzf", filepath.Join("/tmp", source)}, "/"); err != nil {
					return messages, fmt.Errorf("unable to extract archile file %s in the node:\n\t%w", source, err)
				}
			}
		}

		// Start process when necessary
		if n.Type == "router" {
			// start FRR daemon
			if _, err := client.Exec(ctx, n.ID, []string{"/usr/lib/frr/frrinit.sh", "start"}); err != nil {
				return messages, fmt.Errorf("node %s - unable to start router process - %v", n.Name, err)
			}
		} else {
			netconf := path.Join(confPath, n.Name+".net.conf")
			if _, err := os.Stat(netconf); err == nil {
				output, err := client.Exec(ctx, n.ID, []string{"network-config.py", "-l", "/tmp/custom.net.conf"})
				if err != nil {
					return messages, fmt.Errorf("node %s - unable to load network config - %v", n.Name, err)
				} else if output != "" {
					messages = strings.Split(output, "\n")
				}
			}
		}

		// execute init script if it exists
		if client.IsFileExist(ctx, n.ID, initScript) {
			output, err := client.Exec(ctx, n.ID, []string{"sh", initScript})
			if err != nil {
				return messages, err
			} else if output != "" {
				messages = strings.Split(output, "\n")
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

	var configFiles map[string]string
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

	switch n.Type {
	case "host":
		configFiles = map[string]string{
			"Network": fmt.Sprintf("%s.net.conf", n.Name),
			"NTP":     fmt.Sprintf("%s.ntp.conf", n.Name),
		}
	case "server":
		configFiles = map[string]string{
			"Network": fmt.Sprintf("%s.net.conf", n.Name),
			"NTP":     fmt.Sprintf("%s.ntp.conf", n.Name),
		}
	case "router":
		configFiles = map[string]string{
			"FRR": fmt.Sprintf("%s.frr.conf", n.Name),
		}
	}

	for name, filename := range configFiles {
		filepath := path.Join(filesDir, filename)
		if _, err := os.Stat(filepath); os.IsNotExist(err) {
			configFilesData[name] = []byte{}
			continue
		}

		data, err := os.ReadFile(filepath)
		if err != nil {
			return nil, fmt.Errorf("unable to read config file '%s':\n\t%w", filepath, err)
		}
		configFilesData[name] = data
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

	configFiles := make(map[string]string)
	configFolders := make(map[string]string)
	switch n.Type {
	case "host":
		confFile := "/tmp/custom.net.conf"
		if _, err := client.Exec(ctx, n.ID, []string{"network-config.py", "-s", confFile}); err != nil {
			return fmt.Errorf("node %s - unable to save network config - %v", n.Name, err)
		}
		configFiles[confFile] = fmt.Sprintf("%s.net.conf", n.Name)
		configFiles["/etc/ntp.conf"] = fmt.Sprintf("%s.ntp.conf", n.Name)

	case "server":
		confFile := "/tmp/custom.net.conf"
		if _, err := client.Exec(ctx, n.ID, []string{"network-config.py", "-s", confFile}); err != nil {
			return fmt.Errorf("node %s - unable to save network config - %v", n.Name, err)
		}
		configFiles[confFile] = fmt.Sprintf("%s.net.conf", n.Name)
		configFiles["/etc/ntp.conf"] = fmt.Sprintf("%s.ntp.conf", n.Name)
		configFiles["/etc/dhcp/dhcpd.conf"] = fmt.Sprintf("%s.dhcpd.conf", n.Name)
		configFiles["/etc/default/tftpd-hpa"] = fmt.Sprintf("%s.tftpd-hpa.default", n.Name)
		configFiles["/etc/default/isc-dhcp-relay"] = fmt.Sprintf("%s.isc-relay.default", n.Name)
		configFiles["/etc/default/named"] = fmt.Sprintf("%s.bind.default", n.Name)

		configFolders["/srv/tftp"] = fmt.Sprintf("%s.tftp-data.tgz", n.Name)
		configFolders["/etc/bind"] = fmt.Sprintf("%s.bind-etc.tgz", n.Name)

	case "router":
		confFile := "/etc/frr/frr.conf"
		if _, err := client.Exec(ctx, n.ID, []string{"vtysh", "-w"}); err != nil {
			return fmt.Errorf("node %s - unable to save router config - %v", n.Name, err)
		}
		if _, err := client.Exec(ctx, n.ID, []string{"chmod", "+r", confFile}); err != nil {
			return err
		}
		configFiles[confFile] = fmt.Sprintf("%s.frr.conf", n.Name)
	}

	// Save init script if it exists
	configFiles[initScript] = fmt.Sprintf("%s.init.conf", n.Name)
	for source, dest := range configFiles {
		dest = path.Join(dstPath, dest)
		if !client.IsFileExist(ctx, n.ID, source) {
			continue
		}

		if err := client.CopyFrom(ctx, n.ID, source, dest); err != nil {
			return fmt.Errorf("node %s - unable to save file %s:\n\t%v", n.Name, source, err)
		}
	}

	for source, dest := range configFolders {
		destPath := path.Join(dstPath, dest)
		if !client.IsFolderExist(ctx, n.ID, source) {
			continue
		}

		if _, err := client.ExecWithWorkingDir(ctx, n.ID, []string{"tar", "czf", "/tmp/" + dest, source}, "/"); err != nil {
			return err
		}

		if err := client.CopyFrom(ctx, n.ID, "/tmp/"+dest, destPath); err != nil {
			return fmt.Errorf("node %s - unable to save archive file %s:\n\t%v", n.Name, source, err)
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

func NewDockerNode(prjID string, dockerOpts DockerNodeOptions) (*DockerNode, error) {
	node := &DockerNode{
		PrjID:      prjID,
		ID:         "",
		Name:       dockerOpts.Name,
		ShortName:  dockerOpts.ShortName,
		Type:       dockerOpts.Type,
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

	imgName := dockerOpts.ImgName
	if imgName == "" {
		// use default image
		switch dockerOpts.Type {
		case "host":
			imgName = options.GetDockerImageId(options.IMG_HOST)
		case "server":
			imgName = options.GetDockerImageId(options.IMG_SERVER)
		case "router":
			imgName = options.GetDockerImageId(options.IMG_ROUTER)
		default:
			return node, fmt.Errorf("docker type %s is not known", dockerOpts.Type)
		}
	}

	if err := node.Create(imgName, dockerOpts.Ipv6); err != nil {
		return node, err
	}
	return node, nil
}
