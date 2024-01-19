package ovs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/moby/term"
	"github.com/mroy31/gonetem/internal/docker"
	"github.com/mroy31/gonetem/internal/link"
	"github.com/mroy31/gonetem/internal/options"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netns"
)

type OvsNode struct {
	PrjID       string
	Name        string
	ShortName   string
	Running     bool
	OvsInstance *OvsProjectInstance
	Interfaces  map[string]link.IfState
	Logger      *logrus.Entry
}

func (s *OvsNode) GetName() string {
	return s.Name
}

func (s *OvsNode) GetShortName() string {
	if s.ShortName == "" {
		return s.Name
	}
	return s.ShortName
}

func (s *OvsNode) GetType() string {
	return "ovs"
}

func (o *OvsNode) IsRunning() bool {
	return o.Running
}

func (o *OvsNode) CanRunConsole() error {
	if !o.Running {
		return errors.New("Not running")
	}
	return nil
}

func (o *OvsNode) Console(shell bool, in io.ReadCloser, out io.Writer, resizeCh chan term.Winsize) error {
	if !o.Running {
		return errors.New("Not running")
	}

	client, err := docker.NewDockerClient()
	if err != nil {
		return err
	}
	defer client.Close()

	cmd := []string{"/bin/bash"}
	if !shell {
		cmd = []string{"/usr/bin/ovs-console.py", o.GetBridgeName()}
	}

	return client.ExecTty(o.OvsInstance.containerId, cmd, in, out, resizeCh)
}

func (o *OvsNode) CopyFrom(srcPath, destPath string) error {
	return fmt.Errorf("CopyFrom action not supported for ovswitch node")
}

func (o *OvsNode) CopyTo(srcPath, destPath string) error {
	return fmt.Errorf("CopyTo action not supported for ovswitch node")
}

func (o *OvsNode) GetNetns() (netns.NsHandle, error) {
	return o.OvsInstance.GetNetns()
}

func (o *OvsNode) GetBridgeName() string {
	return o.Name
}

func (o *OvsNode) GetInterfaceName(ifIndex int) string {
	return fmt.Sprintf("%s.%d", o.GetBridgeName(), ifIndex)
}

func (o *OvsNode) Capture(ifIndex int, out io.Writer) error {
	if !o.Running {
		return fmt.Errorf("Not running")
	}

	return o.OvsInstance.Capture(o.GetInterfaceName(ifIndex), out)
}

func (o *OvsNode) Start() error {
	if !o.Running {
		if err := o.OvsInstance.AddBr(o.GetBridgeName()); err != nil {
			return err
		}
		o.Running = true

		for ifName := range o.Interfaces {
			if err := o.OvsInstance.AddPort(o.GetBridgeName(), ifName); err != nil {
				return err
			}
		}
	}

	return nil
}

func (o *OvsNode) Stop() error {
	if o.Running {
		for ifName := range o.Interfaces {
			if err := o.OvsInstance.DelPort(o.GetBridgeName(), ifName); err != nil {
				return err
			}
		}

		if err := o.OvsInstance.DelBr(o.GetBridgeName()); err != nil {
			return err
		}
		o.Running = false
	}

	return nil
}

func (o *OvsNode) AddInterface(ifName string, ifIndex int, ns netns.NsHandle) error {
	targetIfName := o.GetInterfaceName(ifIndex)
	if err := link.RenameLink(ifName, targetIfName, ns); err != nil {
		return err
	}

	if err := o.OvsInstance.AddPort(o.GetBridgeName(), targetIfName); err != nil {
		return err
	}

	o.Interfaces[targetIfName] = link.IFSTATE_UP

	return nil
}

func (o *OvsNode) GetInterfacesState() map[string]link.IfState {
	ifStates := make(map[string]link.IfState, 0)
	for ifName, state := range o.Interfaces {
		nArgs := strings.Split(ifName, ".")
		ifStates[nArgs[len(nArgs)-1]] = state
	}

	return ifStates
}

func (n *OvsNode) SetInterfaceState(ifIndex int, state link.IfState) error {
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

func (o *OvsNode) ReadConfigFiles(confDir string) (map[string][]byte, error) {
	configFilesData := make(map[string][]byte)

	filesDir := confDir
	if o.Running {
		// create temp directory for the project
		dir, err := os.MkdirTemp(options.ServerConfig.Workdir, "gonetem-config-sw"+o.Name+"-")
		if err != nil {
			return nil, fmt.Errorf("unable to create temp folder to save ovs config: %w", err)
		}

		err = o.OvsInstance.SaveConfig(o.Name, o.GetBridgeName(), dir)
		if err != nil {
			return nil, fmt.Errorf("unable to save ovs config in temp folder %s: %w", dir, err)
		}
		filesDir = dir

		defer os.RemoveAll(dir)
	}

	confpath := path.Join(filesDir, o.Name+".conf")
	if _, err := os.Stat(confpath); os.IsNotExist(err) {
		configFilesData["OVS"] = []byte{}
		return configFilesData, nil
	}

	data, err := os.ReadFile(confpath)
	if err != nil {
		return nil, fmt.Errorf("unable to read ovs config file '%s':\n\t%w", confpath, err)
	}

	configFilesData["OVS"] = data
	return configFilesData, nil
}

func (o *OvsNode) LoadConfig(confPath string) ([]string, error) {
	if !o.Running {
		o.Logger.Warn("LoadConfig: node not running")
		return []string{}, nil
	}

	return o.OvsInstance.LoadConfig(o.Name, o.GetBridgeName(), confPath)
}

func (o *OvsNode) Save(dstPath string) error {
	if !o.Running {
		o.Logger.Warn("Save: node not running")
		return nil
	}

	return o.OvsInstance.SaveConfig(o.Name, o.GetBridgeName(), dstPath)
}

func (o *OvsNode) Close() error {
	if o.OvsInstance != nil {
		return o.OvsInstance.DelBr(o.GetBridgeName())
	}
	return nil
}

func NewOvsNode(prjID, name, shortName string) (*OvsNode, error) {
	node := &OvsNode{
		PrjID:      prjID,
		Name:       name,
		ShortName:  shortName,
		Interfaces: make(map[string]link.IfState),
		Logger: logrus.WithFields(logrus.Fields{
			"project": prjID,
			"node":    "ovs-" + name,
		}),
	}

	node.OvsInstance = GetOvsInstance(prjID)
	if node.OvsInstance == nil {
		return node, fmt.Errorf("Ovswitch instance for project %s not found", prjID)
	}
	return node, nil
}
