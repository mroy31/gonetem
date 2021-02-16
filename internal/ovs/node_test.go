package ovs

import (
	"testing"

	"github.com/mroy31/gonetem/internal/options"
	"github.com/mroy31/gonetem/internal/utils"
)

func TestOvsNode_StartStop(t *testing.T) {
	options.InitServerConfig()
	prjID := utils.RandString(3)
	name := utils.RandString(4)

	instance, err := NewOvsInstance(prjID)
	if err != nil {
		t.Errorf("Unable to create ovs instance: %v", err)
		return
	}
	defer instance.Close()

	if err := instance.Start(); err != nil {
		t.Errorf("Unable to start ovs instance: %v", err)
		return
	}

	node, err := NewOvsNode(prjID, name)
	if err != nil {
		t.Errorf("Unable to create ovs node: %v", err)
		return
	}
	defer node.Close()

	// Start the node
	if err := node.Start(); err != nil {
		t.Errorf("Unable to start ovs node: %v", err)
	}

	// Stop the node
	if err := node.Stop(); err != nil {
		t.Errorf("Unable to stop ovs node: %v", err)
	}
}
