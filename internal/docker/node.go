package docker

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/moby/term"
	"github.com/mroy31/gonetem/internal/link"
	"github.com/mroy31/gonetem/internal/options"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netns"
)

type DockerNodeOptions struct {
	Name    string
	Type    string
	ImgName string
	Ipv6    bool
	Mpls    bool
}

type DockerNodeStatus struct {
	Running bool
}

type DockerNode struct {
	PrjID          string
	ID             string
	Name           string
	Type           string
	Interfaces     map[string]link.IfState
	LocalNetnsName string
	Running        bool
	ConfigLoaded   bool
	Mpls           bool
	Logger         *logrus.Entry
}

func (n *DockerNode) GetName() string {
	return n.Name
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

	state, err := client.GetState(n.ID)
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
	var err error

	// first created local named ns to detach interface without delete it
	nsName := fmt.Sprintf("%s%s", n.PrjID, n.Name)
	ns, err := netns.NewNamed(nsName)
	if err != nil {
		return fmt.Errorf("Error when creating node netns '%s': %v", nsName, err)
	}
	ns.Close()
	n.LocalNetnsName = nsName

	client, err := NewDockerClient()
	if err != nil {
		return err
	}
	defer client.Close()

	present, err := client.IsImagePresent(imgName)
	if err != nil {
		return err
	} else if !present {
		return fmt.Errorf("Docker image %s not present", imgName)
	}

	containerName := fmt.Sprintf("ntm%s.%s", n.PrjID, n.Name)
	if n.ID, err = client.Create(imgName, containerName, n.Name, ipv6); err != nil {
		return err
	}

	return nil
}

func (n *DockerNode) GetNetns() (netns.NsHandle, error) {
	if !n.Running {
		return netns.NsHandle(0), fmt.Errorf("Node %s Not running", n.GetName())
	}

	client, err := NewDockerClient()
	if err != nil {
		return netns.NsHandle(0), err
	}
	defer client.Close()

	pid, err := client.Pid(n.ID)
	if err != nil {
		return netns.NsHandle(0), err
	}
	return netns.GetFromPid(pid)
}

func (n *DockerNode) GetInterfaceName(ifIndex int) string {
	return fmt.Sprintf("eth%d", ifIndex)
}

func (n *DockerNode) AddInterface(ifIndex int) error {
	n.Interfaces[n.GetInterfaceName(ifIndex)] = link.IFSTATE_UP

	return nil
}

func (n *DockerNode) CanRunConsole() error {
	if !n.Running {
		return errors.New("Not running")
	}
	return nil
}

func (n *DockerNode) Capture(ifIndex int, out io.Writer) error {
	if !n.Running {
		return errors.New("Not running")
	}

	client, err := NewDockerClient()
	if err != nil {
		return err
	}
	defer client.Close()

	cmd := []string{"tcpdump", "-w", "-", "-s", "0", "-U", "-i", n.GetInterfaceName(ifIndex)}
	return client.ExecOutStream(n.ID, cmd, out)
}

func (n *DockerNode) Console(in io.ReadCloser, out io.Writer, resizeCh chan term.Winsize) error {
	if !n.Running {
		return errors.New("Not running")
	}

	client, err := NewDockerClient()
	if err != nil {
		return err
	}
	defer client.Close()

	var cmd []string
	switch n.Type {
	case "router":
		cmd = []string{"/usr/bin/vtysh"}
	default:
		cmd = []string{"/bin/bash"}
	}

	return client.ExecTty(n.ID, cmd, in, out, resizeCh)
}

func (n *DockerNode) Start() error {
	if !n.Running {
		n.Logger.Debug("Start Node")

		client, err := NewDockerClient()
		if err != nil {
			return err
		}
		defer client.Close()

		if err := client.Start(n.ID); err != nil {
			return err
		}
		n.Running = true

		// Attach existing interfaces
		currentNS, err := netns.GetFromName(n.LocalNetnsName)
		if err != nil {
			return fmt.Errorf("Unable to get netns associated to node %s: %v", n.Name, err)
		}
		defer currentNS.Close()

		targetNS, err := n.GetNetns()
		if err != nil {
			return err
		}
		defer targetNS.Close()

		if err := link.MoveInterfacesNetns(n.Interfaces, currentNS, targetNS); err != nil {
			return fmt.Errorf("Unable to attach interfaces: %v", err)
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
		currentNS, err := n.GetNetns()
		if err != nil {
			return fmt.Errorf("Unable to get netns associated to node %s: %v", n.Name, err)
		}
		defer currentNS.Close()

		targetNS, err := netns.GetFromName(n.LocalNetnsName)
		if err != nil {
			return err
		}
		defer targetNS.Close()

		if err := link.MoveInterfacesNetns(n.Interfaces, currentNS, targetNS); err != nil {
			return fmt.Errorf("Unable to attach interfaces: %v", err)
		}

		if err := client.Stop(n.ID); err != nil {
			return err
		}
		n.Running = false
		n.ConfigLoaded = false

	}

	return nil
}

func (n *DockerNode) LoadConfig(confPath string) error {
	if !n.Running {
		n.Logger.Warn("LoadConfig: node not running")
		return nil
	}

	if !n.ConfigLoaded {
		client, err := NewDockerClient()
		if err != nil {
			return err
		}
		defer client.Close()

		if _, err := os.Stat(confPath); err == nil {
			configFiles := make(map[string]string)
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
			}

			for source, dest := range configFiles {
				source = path.Join(confPath, source)
				if _, err := os.Stat(source); os.IsNotExist(err) {
					continue
				}

				if err := client.CopyTo(n.ID, source, dest); err != nil {
					return fmt.Errorf("Unable to load config file %s:\n\t%v", source, err)
				}
			}

		}

		// Start process when necessary
		if n.Type == "router" {
			// start FRR daemon
			if _, err := client.Exec(n.ID, []string{"/usr/lib/frr/frrinit.sh", "start"}); err != nil {
				return err
			}
		} else {
			netconf := path.Join(confPath, n.Name+".net.conf")
			if _, err := os.Stat(netconf); err == nil {
				if _, err := client.Exec(n.ID, []string{"network-config.py", "-l", "/tmp/custom.net.conf"}); err != nil {
					return err
				}
			}
		}

		n.ConfigLoaded = true
	}

	return nil
}

func (n *DockerNode) Save(dstPath string) error {
	if !n.Running || !n.ConfigLoaded {
		n.Logger.Warn("Save: node not running")
		return nil
	}

	client, err := NewDockerClient()
	if err != nil {
		return err
	}
	defer client.Close()

	configFiles := make(map[string]string)
	switch n.Type {
	case "host":
		confFile := "/tmp/custom.net.conf"
		if _, err := client.Exec(n.ID, []string{"network-config.py", "-s", confFile}); err != nil {
			return err
		}
		configFiles[confFile] = fmt.Sprintf("%s.net.conf", n.Name)
		configFiles["/etc/ntp.conf"] = fmt.Sprintf("%s.ntp.conf", n.Name)

	case "server":
		confFile := "/tmp/custom.net.conf"
		if _, err := client.Exec(n.ID, []string{"network-config.py", "-s", confFile}); err != nil {
			return err
		}
		configFiles[confFile] = fmt.Sprintf("%s.net.conf", n.Name)
		configFiles["/etc/ntp.conf"] = fmt.Sprintf("%s.ntp.conf", n.Name)
		configFiles["/etc/dhcp/dhcpd.conf"] = fmt.Sprintf("%s.dhcpd.conf", n.Name)
		configFiles["/etc/default/tftpd-hpa"] = fmt.Sprintf("%s.tftpd-hpa.default", n.Name)

	case "router":
		confFile := "/etc/frr/frr.conf"
		if _, err := client.Exec(n.ID, []string{"vtysh", "-w"}); err != nil {
			return err
		}
		if _, err := client.Exec(n.ID, []string{"chmod", "+r", confFile}); err != nil {
			return err
		}
		configFiles[confFile] = fmt.Sprintf("%s.frr.conf", n.Name)
	}

	for source, dest := range configFiles {
		dest = path.Join(dstPath, dest)
		if err := client.CopyFrom(n.ID, source, dest); err != nil {
			msg := fmt.Sprintf("Unable to save file %s:\n\t%v", source, err)
			return errors.New(msg)
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

	return client.CopyFrom(n.ID, source, dest)
}

func (n *DockerNode) CopyTo(source, dest string) error {
	client, err := NewDockerClient()
	if err != nil {
		return err
	}
	defer client.Close()

	return client.CopyTo(n.ID, source, dest)
}

func (n *DockerNode) GetInterfaces() map[string]link.IfState {
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

	return fmt.Errorf("Interface %s.%d not found", n.GetName(), ifIndex)
}

func (n *DockerNode) Close() error {
	if n.ID != "" {
		n.Logger.Debug("Close node")

		client, err := NewDockerClient()
		if err != nil {
			return err
		}
		defer client.Close()

		if n.Running {
			client.Stop(n.ID)
			n.Running = false
		}

		if err := client.Rm(n.ID); err != nil {
			return err
		}

		// clean attributes
		n.ConfigLoaded = false
		n.Interfaces = make(map[string]link.IfState)
		netns.DeleteNamed(n.LocalNetnsName)
	}

	return nil
}

func NewDockerNode(prjID string, dockerOpts DockerNodeOptions) (*DockerNode, error) {
	imgName := dockerOpts.ImgName
	if imgName == "" {
		// use default image
		switch dockerOpts.Type {
		case "host":
			imgName = fmt.Sprintf(
				"%s:%s",
				options.ServerConfig.Docker.Images.Host, options.VERSION)
		case "server":
			imgName = fmt.Sprintf(
				"%s:%s",
				options.ServerConfig.Docker.Images.Server, options.VERSION)
		case "router":
			imgName = fmt.Sprintf(
				"%s:%s",
				options.ServerConfig.Docker.Images.Router, options.VERSION)
		default:
			return nil, errors.New(fmt.Sprintf("Docker type %s is not known", dockerOpts.Type))
		}
	}

	node := &DockerNode{
		PrjID:      prjID,
		ID:         "",
		Name:       dockerOpts.Name,
		Type:       dockerOpts.Type,
		Mpls:       dockerOpts.Mpls,
		Interfaces: make(map[string]link.IfState),
		Logger: logrus.WithFields(logrus.Fields{
			"project": prjID,
			"node":    dockerOpts.Name,
		}),
	}

	if err := node.Create(imgName, dockerOpts.Ipv6); err != nil {
		return node, err
	}
	return node, nil
}
