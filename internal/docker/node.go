package docker

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/moby/term"
	"github.com/mroy31/gonetem/internal/logger"
	"github.com/mroy31/gonetem/internal/options"
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
	PrjID        string
	ID           string
	Name         string
	Type         string
	Running      bool
	ConfigLoaded bool
	Mpls         bool
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
	client, err := NewDockerClient()
	if err != nil {
		return err
	}
	defer client.Close()

	containerName := fmt.Sprintf("%s.%s", n.PrjID, n.Name)
	if n.ID, err = client.Create(imgName, containerName, n.Name, ipv6); err != nil {
		return err
	}

	return nil
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
		client, err := NewDockerClient()
		if err != nil {
			return err
		}
		defer client.Close()

		if err := client.Start(n.ID); err != nil {
			return err
		}
		n.Running = true
	}

	return nil
}

func (n *DockerNode) Stop() error {
	if n.Running {
		client, err := NewDockerClient()
		if err != nil {
			return err
		}
		defer client.Close()

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
		logger.Warn("msg", "LoadConfig: node not running", "node", n.Name)
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
		}

		n.ConfigLoaded = true
	}

	return nil
}

func (n *DockerNode) Save(dstPath string) error {
	if !n.Running || !n.ConfigLoaded {
		logger.Warn("msg", "Save: node not running", "node", n.Name)
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
	if !n.Running {
		logger.Warn("msg", "CopyFrom: node not running", "node", n.Name)
		return nil
	}

	client, err := NewDockerClient()
	if err != nil {
		return err
	}
	defer client.Close()

	return client.CopyFrom(n.ID, source, dest)
}

func (n *DockerNode) CopyTo(source, dest string) error {
	if !n.Running {
		logger.Warn("msg", "CopyTo: node not running", "node", n.Name)
		return nil
	}

	client, err := NewDockerClient()
	if err != nil {
		return err
	}
	defer client.Close()

	return client.CopyTo(n.ID, source, dest)
}

func (n *DockerNode) Close() error {
	if n.ID != "" {
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
	}

	return nil
}

func CreateDockerNode(prjID string, dockerOpts DockerNodeOptions) (*DockerNode, error) {
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
		PrjID: prjID,
		ID:    "",
		Name:  dockerOpts.Name,
		Type:  dockerOpts.Type,
		Mpls:  dockerOpts.Mpls,
	}

	if err := node.Create(imgName, dockerOpts.Ipv6); err != nil {
		return node, err
	}
	return node, nil
}
