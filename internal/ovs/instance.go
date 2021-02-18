package ovs

import (
	"fmt"

	"github.com/mroy31/gonetem/internal/docker"
	"github.com/mroy31/gonetem/internal/options"
)

type State int

const (
	created State = 1 << iota
	started
)

var (
	ovsInstances = make(map[string]*OvsProjectInstance)
)

type OvsProjectInstance struct {
	prjID       string
	containerId string
	state       State
	bridges     []string
}

func (o *OvsProjectInstance) Start() error {
	if o.state != started {
		client, err := docker.NewDockerClient()
		if err != nil {
			return err
		}
		defer client.Close()

		if err := client.Start(o.containerId); err != nil {
			return err
		}
		o.state = started
	}

	return nil
}

func (o *OvsProjectInstance) findBr(brName string) int {
	for idx, br := range o.bridges {
		if br == brName {
			return idx
		}
	}
	return -1
}

func (o *OvsProjectInstance) AddBr(brName string) error {
	if o.findBr(brName) != -1 {
		return fmt.Errorf("Switch %s already exists", brName)
	}

	client, err := docker.NewDockerClient()
	if err != nil {
		return err
	}
	defer client.Close()

	cmd := []string{"ovs-vsctl", "add-br", brName}
	if _, err := client.Exec(o.containerId, cmd); err != nil {
		return err
	}
	o.bridges = append(o.bridges, brName)

	return nil
}

func (o *OvsProjectInstance) DelBr(brName string) error {
	brIdx := o.findBr(brName)
	if brIdx != -1 {
		client, err := docker.NewDockerClient()
		if err != nil {
			return err
		}
		defer client.Close()

		cmd := []string{"ovs-vsctl", "del-br", brName}
		if _, err := client.Exec(o.containerId, cmd); err != nil {
			return err
		}

		// remove br from list
		o.bridges[brIdx] = o.bridges[len(o.bridges)-1]
		o.bridges = o.bridges[:len(o.bridges)-1]
	}

	return nil
}

func (o *OvsProjectInstance) AttachInterface(brName, ifName string, ifIndex int) error {
	client, err := docker.NewDockerClient()
	if err != nil {
		return err
	}
	defer client.Close()

	name := fmt.Sprintf("%s.%d", brName, ifIndex)
	if err := client.AttachInterface(o.containerId, ifName, name); err != nil {
		return err
	}

	cmd := []string{"ovs-vsctl", "add-port", brName, name}
	if _, err := client.Exec(o.containerId, cmd); err != nil {
		return err
	}

	return nil
}

func (o *OvsProjectInstance) Close() error {
	client, err := docker.NewDockerClient()
	if err != nil {
		return err
	}
	defer client.Close()

	if o.state == started {
		client.Stop(o.containerId)
	}

	if err := client.Rm(o.containerId); err != nil {
		return err
	}

	return nil
}

func NewOvsInstance(prjID string) (*OvsProjectInstance, error) {
	_, ok := ovsInstances[prjID]
	if ok {
		return nil, fmt.Errorf("ovswitch container already exists")
	}

	client, err := docker.NewDockerClient()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	imgName := options.ServerConfig.Docker.Images.Ovs + ":" + options.VERSION
	present, err := client.IsImagePresent(imgName)
	if err != nil {
		return nil, err
	} else if !present {
		return nil, fmt.Errorf("Ovswitch image %s not present", imgName)
	}

	containerName := fmt.Sprintf("ntm%s.ovs", prjID)
	containerId, err := client.Create(imgName, containerName, "ovs", false)
	if err != nil {
		return nil, err
	}

	ovsInstances[prjID] = &OvsProjectInstance{
		prjID:       prjID,
		containerId: containerId,
		state:       created,
	}

	return ovsInstances[prjID], nil
}

func GetOvsInstance(prjID string) *OvsProjectInstance {
	instance, ok := ovsInstances[prjID]
	if ok {
		return instance
	}
	return nil
}

func CloseOvsInstance(prjID string) error {
	ovs := GetOvsInstance(prjID)
	if ovs == nil {
		return fmt.Errorf("ovswitch container not found")
	}

	defer delete(ovsInstances, prjID)
	return ovs.Close()
}
