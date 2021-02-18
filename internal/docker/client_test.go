package docker

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/mroy31/gonetem/internal/link"
	"github.com/mroy31/gonetem/internal/options"
	"github.com/mroy31/gonetem/internal/utils"
	"github.com/vishvananda/netlink"
)

func initClient() (*DockerClient, error) {
	options.InitServerConfig()
	return NewDockerClient()
}

func TestDockerClient_ImagePresent(t *testing.T) {
	client, err := initClient()
	if err != nil {
		t.Errorf("Unable to init docker client: %v", err)
		return
	}

	tests := []struct {
		name    string
		imgName string
		result  bool
	}{
		{
			name:    "ImagePresent: valid test",
			imgName: fmt.Sprintf("mroy31/pynetem-host:%s", options.VERSION),
			result:  true,
		},
		{
			name:    "ImagePresent: invalid test with random string",
			imgName: utils.RandString(10),
			result:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			present, err := client.IsImagePresent(tt.imgName)
			if err != nil {
				t.Errorf("IsImagePresent returns an error: %v", err)
			} else if present != tt.result {
				t.Errorf("IsImagePresent returns wrong result: %v != %v", present, tt.result)
			}
		})
	}
}

func TestDockerClient_CreateRm(t *testing.T) {
	client, err := initClient()
	if err != nil {
		t.Errorf("Unable to init docker client: %v", err)
		return
	}

	// create a container
	img := fmt.Sprintf("%s:%s", options.ServerConfig.Docker.Images.Host, options.VERSION)
	name := utils.RandString(10)
	cID, err := client.Create(img, name, name, true)
	if err != nil {
		t.Errorf("Unable to create the container: %v", err)
		return
	}

	// check the existence of the container
	container, err := client.Get(cID)
	if err != nil {
		t.Errorf("Unable to get info about the created container: %v", err)
	} else if container == nil {
		t.Error("The container has not been created")
	}

	// start the container
	if err = client.Start(cID); err != nil {
		t.Errorf("Unable to start the container: %v", err)
	}

	// check the status of the container
	container, err = client.Get(cID)
	if err != nil {
		t.Errorf("Unable to get info about the created container: %v", err)
	}
	if container.State != "running" {
		t.Errorf("The container is not started %s != 'running'", container.State)
	}

	// stop the container
	if err = client.Stop(cID); err != nil {
		t.Errorf("Unable to stop the container: %v", err)
	}

	// remove the container
	if err = client.Rm(cID); err != nil {
		t.Errorf("Unable to remove the container: %v", err)
	}
}

func TestDockerClient_Exec(t *testing.T) {
	client, err := initClient()
	if err != nil {
		t.Errorf("Unable to init docker client: %v", err)
		return
	}

	// create and start a container
	img := fmt.Sprintf("%s:%s", options.ServerConfig.Docker.Images.Server, options.VERSION)
	name := utils.RandString(10)
	cID, err := client.Create(img, name, name, true)
	if err != nil {
		t.Errorf("Unable to create the container: %v", err)
		return
	}
	if err = client.Start(cID); err != nil {
		t.Errorf("Unable to start the container: %v", err)
	}

	// get the hostname
	hostname, err := client.Exec(cID, []string{"hostname"})
	if err != nil {
		t.Errorf("Unable to get the hostname with exec call: %v", err)
	}
	if strings.TrimRight(hostname, "\n") != name {
		t.Errorf("The hostname is not correct: %s != %s", hostname, name)
	}

	// stop and rm
	if err = client.Stop(cID); err != nil {
		t.Errorf("Unable to stop the container: %v", err)
	}
	if err = client.Rm(cID); err != nil {
		t.Errorf("Unable to remove the container: %v", err)
	}
}

func TestDockerClient_Copy(t *testing.T) {
	client, err := initClient()
	if err != nil {
		t.Errorf("Unable to init docker client: %v", err)
		return
	}

	// create a container
	img := fmt.Sprintf("%s:%s", options.ServerConfig.Docker.Images.Router, options.VERSION)
	name := utils.RandString(10)
	cID, err := client.Create(img, name, name, true)
	if err != nil {
		t.Errorf("Unable to create the container: %v", err)
		return
	}

	// create a file and copy it in the container
	tmpData := utils.RandString(32)
	tmpFilename := fmt.Sprintf("/tmp/%s.temp", utils.RandString(12))
	err = ioutil.WriteFile(tmpFilename, []byte(tmpData), 0644)
	if err != nil {
		t.Errorf("Unable to create a temp file: %v", err)
	}
	defer os.Remove(tmpFilename)

	err = client.CopyTo(cID, tmpFilename, "/tmp/test.temp")
	if err != nil {
		t.Errorf("Unable to copy file to container: %v", err)
	}

	// copy the same file from the container
	newTmpFilename := fmt.Sprintf("/tmp/%s.temp", utils.RandString(12))
	err = client.CopyFrom(cID, "/tmp/test.temp", newTmpFilename)
	if err != nil {
		t.Errorf("Unable to copy file from the container: %v", err)
	}
	defer os.Remove(newTmpFilename)

	// check the content of the new file
	data, err := ioutil.ReadFile(newTmpFilename)
	if err != nil {
		t.Errorf("Unable to read the new temp file: %v", err)
	}
	if string(data) != tmpData {
		t.Errorf("Data are different between CopyTo/CopyFrom: %s != %s", string(data), tmpData)
	}

	// clean up
	if err = client.Rm(cID); err != nil {
		t.Errorf("Unable to remove the container: %v", err)
	}
}

func TestDockerClient_Pid(t *testing.T) {
	client, err := initClient()
	if err != nil {
		t.Errorf("Unable to init docker client: %v", err)
		return
	}

	// create a container
	img := fmt.Sprintf("%s:%s", options.ServerConfig.Docker.Images.Router, options.VERSION)
	name := utils.RandString(10)
	cID, err := client.Create(img, name, name, true)
	if err != nil {
		t.Errorf("Unable to create the container: %v", err)
		return
	}

	// pid must be equal to -1
	pid, _ := client.Pid(cID)
	if pid != -1 {
		t.Errorf("Container is not running but Pid != -1, Pid == %d", pid)
	}

	// start container
	if err = client.Start(cID); err != nil {
		t.Errorf("Unable to start the container: %v", err)
	}

	// pid must be different to -1
	pid, _ = client.Pid(cID)
	if pid == -1 {
		t.Error("Container is running but Pid == -1", pid)
	}

	// stop and rm
	if err = client.Stop(cID); err != nil {
		t.Errorf("Unable to stop the container: %v", err)
	}
	if err = client.Rm(cID); err != nil {
		t.Errorf("Unable to remove the container: %v", err)
	}
}

func TestDockerClient_AttachInterface(t *testing.T) {
	client, err := initClient()
	if err != nil {
		t.Errorf("Unable to init docker client: %v", err)
		return
	}

	// create a container
	img := fmt.Sprintf("%s:%s", options.ServerConfig.Docker.Images.Router, options.VERSION)
	name := utils.RandString(10)
	cID, err := client.Create(img, name, name, true)
	if err != nil {
		t.Errorf("Unable to create the container: %v", err)
		return
	}
	defer func() {
		client.Stop(cID)
		client.Rm(cID)
	}()

	// start container
	if err = client.Start(cID); err != nil {
		t.Errorf("Unable to start the container: %v", err)
		return
	}

	// create veth if
	myVeth, err := link.CreateVethLink(utils.RandString(4), utils.RandString(4))
	if err != nil {
		t.Errorf("Unable to create veth: %v", err)
		return
	}
	defer netlink.LinkDel(myVeth)

	if err := client.AttachInterface(cID, myVeth.PeerName, "eth0"); err != nil {
		t.Errorf("Unable to attach if %s to container: %v", myVeth.PeerName, err)
		return
	}
}
