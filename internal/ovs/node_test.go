package ovs

import (
	"testing"

	"github.com/mroy31/gonetem/internal/link"
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

func TestOvsNode_AttachLink(t *testing.T) {
	options.InitServerConfig()
	prjID := utils.RandString(3)

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

	node1, err := NewOvsNode(prjID, utils.RandString(4))
	if err != nil {
		t.Errorf("Unable to create ovs node: %v", err)
		return
	}
	defer node1.Close()

	node2, err := NewOvsNode(prjID, utils.RandString(4))
	if err != nil {
		t.Errorf("Unable to create ovs node: %v", err)
		return
	}
	defer node2.Close()

	for _, node := range []*OvsNode{node1, node2} {
		if err := node.Start(); err != nil {
			t.Fatalf("Unable to start ovs node %s: %v", node.GetName(), err)
		}
	}

	// create link
	node1Netns, err := node1.GetNetns()
	if err != nil {
		t.Fatalf("Unable to get netns for node 1: %v", err)
	}
	defer node1Netns.Close()

	node2Netns, err := node2.GetNetns()
	if err != nil {
		t.Fatalf("Unable to get netns for node 2: %v", err)
	}
	defer node2Netns.Close()

	_, err = link.CreateVethLink(
		node1.GetInterfaceName(0), node1Netns,
		node2.GetInterfaceName(0), node2Netns,
	)
	if err != nil {
		t.Fatalf("Unable to create veth: %v", err)
	}
}
