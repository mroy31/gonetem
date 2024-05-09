package ovs

import (
	"context"
	"testing"

	"github.com/mroy31/gonetem/internal/docker"
	"github.com/mroy31/gonetem/internal/options"
	"github.com/mroy31/gonetem/internal/utils"
)

func TestOVS_Instance(t *testing.T) {
	options.InitServerConfig()
	prjID := utils.RandString(4)

	instance := GetOvsInstance(prjID)
	if instance != nil {
		t.Errorf("Instance for prjID %s seems to exist", prjID)
		return
	}

	instance, err := NewOvsInstance(prjID)
	if err != nil {
		t.Errorf("Unable to create ovs instance: %v", err)
		return
	}
	defer instance.Close()

	// check existence of container
	client, err := docker.NewDockerClient()
	if err != nil {
		t.Errorf("Unable to init docker client: %v", err)
		return
	}
	defer client.Close()

	c, err := client.GetState(context.Background(), instance.containerId)
	if err != nil {
		t.Errorf("Unable to get state of ovs instance: %v", err)
		return
	}
	if c != "created" {
		t.Errorf("Bad state for ovs instance: %s != created", c)
		return
	}

	// start ovs instance and check
	if err := instance.Start(); err != nil {
		t.Errorf("Unable to start ovs instance: %v", err)
		return
	}
	c, err = client.GetState(context.Background(), instance.containerId)
	if err != nil {
		t.Errorf("Unable to get state of ovs instance: %v", err)
		return
	}
	if c != "running" {
		t.Errorf("Bad state for ovs instance: %s != running", c)
		return
	}
}
