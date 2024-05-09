package ovs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/mroy31/gonetem/internal/docker"
	"github.com/mroy31/gonetem/internal/options"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netns"
)

type State int

const (
	created State = 1 << iota
	started
)

const (
	maxAttempts = 10
)

var (
	ovsInstances = make(map[string]*OvsProjectInstance)
	mutex        = &sync.Mutex{}
)

type OvsProjectInstance struct {
	prjID       string
	containerId string
	state       State
	bridges     []string
	Logger      *logrus.Entry
}

func (o *OvsProjectInstance) Start() error {
	if o.state != started {
		client, err := docker.NewDockerClient()
		if err != nil {
			return err
		}
		defer client.Close()

		ctx := context.Background()
		if err := client.Start(ctx, o.containerId); err != nil {
			return err
		}

		// wait that ovs server has been ready before return
		attempt := 0
		infoCmd := []string{"ovs-vsctl", "show"}
		for attempt < maxAttempts {
			o.Logger.Debugf("Wait ovs-server is ready - attempt %d\n", attempt)
			if err = o.Exec(infoCmd); err == nil {
				break
			}
			attempt++
			time.Sleep(100 * time.Millisecond)
		}
		if err != nil {
			return err
		}
		o.state = started
	}

	return nil
}

func (o *OvsProjectInstance) GetNetns() (netns.NsHandle, error) {
	if o.state != started {
		return netns.NsHandle(0), fmt.Errorf("ovswitch instance not running")
	}

	client, err := docker.NewDockerClient()
	if err != nil {
		return netns.NsHandle(0), err
	}
	defer client.Close()

	pid, err := client.Pid(context.Background(), o.containerId)
	if err != nil {
		return netns.NsHandle(0), err
	}
	return netns.GetFromPid(pid)
}

func (o *OvsProjectInstance) Capture(ifName string, out io.Writer) error {
	if o.state != started {
		return fmt.Errorf("ovswitch instance not running")
	}

	client, err := docker.NewDockerClient()
	if err != nil {
		return err
	}
	defer client.Close()

	cmd := []string{"tcpdump", "-w", "-", "-s", "0", "-U", "-i", ifName}
	return client.ExecOutStream(context.Background(), o.containerId, cmd, out)
}

func (o *OvsProjectInstance) Exec(cmd []string) error {
	client, err := docker.NewDockerClient()
	if err != nil {
		return err
	}
	defer client.Close()

	ctx := context.Background()
	if _, err := client.Exec(ctx, o.containerId, cmd); err != nil {
		return err
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
	mutex.Lock()
	defer mutex.Unlock()

	if o.findBr(brName) != -1 {
		return fmt.Errorf("switch %s already exists", brName)
	}

	cmd := []string{"ovs-vsctl", "add-br", brName}
	if err := o.Exec(cmd); err != nil {
		return err
	}
	o.bridges = append(o.bridges, brName)

	return nil
}

func (o *OvsProjectInstance) DelBr(brName string) error {
	mutex.Lock()
	defer mutex.Unlock()

	brIdx := o.findBr(brName)
	if brIdx != -1 {
		cmd := []string{"ovs-vsctl", "del-br", brName}
		if err := o.Exec(cmd); err != nil {
			return err
		}

		// remove br from list
		o.bridges[brIdx] = o.bridges[len(o.bridges)-1]
		o.bridges = o.bridges[:len(o.bridges)-1]
	}

	return nil
}

func (o *OvsProjectInstance) AddPort(brName, ifName string) error {
	cmd := []string{"ovs-vsctl", "add-port", brName, ifName}
	return o.Exec(cmd)
}

func (o *OvsProjectInstance) DelPort(brName, ifName string) error {
	cmd := []string{"ovs-vsctl", "del-port", brName, ifName}
	return o.Exec(cmd)
}

func (o *OvsProjectInstance) LoadConfig(name, brName, confPath string, timeout int) ([]string, error) {
	var messages []string

	client, err := docker.NewDockerClient()
	if err != nil {
		return messages, err
	}
	defer client.Close()

	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}

	tmpConfFile := "/tmp/" + name + ".conf"
	confFile := path.Join(confPath, name+".conf")
	if _, err := os.Stat(confFile); os.IsNotExist(err) {
		return messages, nil
	}

	if err := client.CopyTo(ctx, o.containerId, confFile, tmpConfFile); err != nil {
		return messages, fmt.Errorf("unable to copy config file %s:\n\t%w", confFile, err)
	}

	cmd := []string{"ovs-config.py", "-a", "load", "-c", tmpConfFile, brName}
	output, err := client.Exec(ctx, o.containerId, cmd)
	if err != nil {
		return messages, fmt.Errorf("unable to load config file %s:\n\t%w", confFile, err)
	} else if output != "" {
		messages = strings.Split(output, "\n")
	}

	return messages, nil
}

func (o *OvsProjectInstance) SaveConfig(name, brName, dstPath string, timeout int) error {
	client, err := docker.NewDockerClient()
	if err != nil {
		return err
	}
	defer client.Close()

	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}

	tmpConfFile := "/tmp/" + name + ".conf"
	confFile := path.Join(dstPath, name+".conf")

	cmd := []string{"ovs-config.py", "-a", "save", "-c", tmpConfFile, brName}
	if _, err := client.Exec(ctx, o.containerId, cmd); err != nil {
		return fmt.Errorf("unable to save config in file %s:\n\t%w", confFile, err)
	}

	if err := client.CopyFrom(ctx, o.containerId, tmpConfFile, confFile); err != nil {
		msg := fmt.Sprintf("Unable to save file %s:\n\t%v", confFile, err)
		return errors.New(msg)
	}

	return nil
}

func (o *OvsProjectInstance) Close() error {
	client, err := docker.NewDockerClient()
	if err != nil {
		return err
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if o.state == started {
		if err := client.Stop(ctx, o.containerId); err != nil {
			return err
		}
	}

	if err := client.Rm(ctx, o.containerId); err != nil {
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

	imgName := options.GetDockerImageId(options.IMG_OVS)
	present, err := client.IsImagePresent(context.Background(), imgName)
	if err != nil {
		return nil, err
	} else if !present {
		return nil, fmt.Errorf("ovswitch image %s not present", imgName)
	}

	containerName := fmt.Sprintf("%s%s.ovs", options.NETEM_ID, prjID)
	containerId, err := client.Create(
		context.Background(),
		imgName,
		containerName,
		"ovs",
		[]string{},
		false,
		false)
	if err != nil {
		return nil, err
	}

	ovsInstances[prjID] = &OvsProjectInstance{
		prjID:       prjID,
		containerId: containerId,
		state:       created,
		Logger: logrus.WithFields(logrus.Fields{
			"project": prjID,
			"node":    "ovs-instance",
		}),
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
