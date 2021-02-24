package ovs

import (
	"fmt"
	"io"

	"github.com/moby/term"
	"github.com/mroy31/gonetem/internal/link"
	"github.com/vishvananda/netns"
)

type OvsNode struct {
	PrjID       string
	Name        string
	Running     bool
	OvsInstance *OvsProjectInstance
	Interfaces  map[string]link.IfState
}

func (s *OvsNode) GetName() string {
	return s.Name
}

func (s *OvsNode) GetType() string {
	return "ovs"
}

func (o *OvsNode) IsRunning() bool {
	return o.Running
}

func (o *OvsNode) CanRunConsole() error {
	return fmt.Errorf("Console not supported for ovswitch node")
}

func (o *OvsNode) Console(shell bool, in io.ReadCloser, out io.Writer, resizeCh chan term.Winsize) error {
	return fmt.Errorf("Console not supported for ovswitch node")
}

func (o *OvsNode) GetNetns() (netns.NsHandle, error) {
	return o.OvsInstance.GetNetns()
}

func (o *OvsNode) GetInterfaceName(ifIndex int) string {
	return fmt.Sprintf("%s.%d", o.Name, ifIndex)
}

func (o *OvsNode) Capture(ifIndex int, out io.Writer) error {
	if !o.Running {
		return fmt.Errorf("Not running")
	}

	return o.OvsInstance.Capture(o.GetInterfaceName(ifIndex), out)
}

func (o *OvsNode) Start() error {
	if !o.Running {
		if err := o.OvsInstance.AddBr(o.Name); err != nil {
			return err
		}
		o.Running = true

		for ifName, _ := range o.Interfaces {
			if err := o.OvsInstance.AddPort(o.Name, ifName); err != nil {
				return err
			}
		}
	}

	return nil
}

func (o *OvsNode) Stop() error {
	if o.Running {
		for ifName, _ := range o.Interfaces {
			if err := o.OvsInstance.DelPort(o.Name, ifName); err != nil {
				return err
			}
		}

		if err := o.OvsInstance.DelBr(o.Name); err != nil {
			return err
		}
		o.Running = false
	}

	return nil
}

func (o *OvsNode) AddInterface(ifIndex int) error {
	if err := o.OvsInstance.AddPort(o.Name, o.GetInterfaceName(ifIndex)); err != nil {
		return err
	}

	o.Interfaces[o.GetInterfaceName(ifIndex)] = link.IFSTATE_UP

	return nil
}

func (o *OvsNode) GetInterfaces() map[string]link.IfState {
	return o.Interfaces
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

func (o *OvsNode) LoadConfig(confPath string) error {
	return nil
}

func (o *OvsNode) Save(dstPath string) error {
	return nil
}

func (o *OvsNode) Close() error {
	if o.OvsInstance != nil {
		return o.OvsInstance.DelBr(o.Name)
	}
	return nil
}

func NewOvsNode(prjID, name string) (*OvsNode, error) {
	node := &OvsNode{
		PrjID:      prjID,
		Name:       name,
		Interfaces: make(map[string]link.IfState),
	}

	node.OvsInstance = GetOvsInstance(prjID)
	if node.OvsInstance == nil {
		return node, fmt.Errorf("Ovswitch instance for project %s not found", prjID)
	}
	return node, nil
}
