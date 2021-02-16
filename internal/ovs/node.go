package ovs

import (
	"fmt"
	"io"

	"github.com/moby/term"
)

type OvsNode struct {
	PrjID       string
	Name        string
	Running     bool
	OvsInstance *OvsProjectInstance
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

func (o *OvsNode) Console(in io.ReadCloser, out io.Writer, resizeCh chan term.Winsize) error {
	return fmt.Errorf("Console not supported for ovswitch node")
}

func (o *OvsNode) Start() error {
	if !o.Running {
		o.Running = true
		return o.OvsInstance.AddBr(o.Name)
	}

	return nil
}

func (o *OvsNode) Stop() error {
	return nil
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
		PrjID: prjID,
		Name:  name,
	}

	node.OvsInstance = GetOvsInstance(prjID)
	if node.OvsInstance == nil {
		return node, fmt.Errorf("Ovswitch instance for project %s not found", prjID)
	}
	return node, nil
}
