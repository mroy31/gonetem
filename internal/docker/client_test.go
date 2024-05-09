package docker

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mroy31/gonetem/internal/options"
	"github.com/mroy31/gonetem/internal/utils"
)

func setupClient(t *testing.T) (*DockerClient, func()) {
	options.InitServerConfig()
	client, err := NewDockerClient()
	if err != nil {
		t.Fatalf("Unable to init docker client: %v", err)
	}

	return client, func() { client.Close() }
}

func setupContainer(t *testing.T, imgId options.DockerImageT) (*DockerClient, string, string, func()) {
	options.InitServerConfig()
	client, _ := NewDockerClient()

	ctx := context.Background()
	image := options.GetDockerImageId(imgId)
	name := utils.RandString(10)

	cID, err := client.Create(ctx, image, name, name, []string{}, true, true)
	if err != nil {
		t.Fatalf("Unable to create the container: %v", err)
	}

	return client, cID, name, func() {
		if err := client.Rm(ctx, cID); err != nil {
			t.Errorf("Unable to rm container %s: %v", cID, err)
		}
		client.Close()
	}
}

func setupStartedContainer(t *testing.T, imgId options.DockerImageT) (*DockerClient, string, string, func()) {
	client, cID, name, teardown := setupContainer(t, imgId)

	if err := client.Start(context.Background(), cID); err != nil {
		client.Rm(context.Background(), cID)
		t.Fatalf("Unable to start the container: %v", err)
	}

	return client, cID, name, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := client.Stop(ctx, cID); err != nil {
			t.Errorf("Unable to stop the container: %v", err)
		}
		teardown()
	}
}

func TestDockerClient_ImagePresent(t *testing.T) {
	client, teardown := setupClient(t)
	defer teardown()

	tests := []struct {
		name    string
		imgName string
		result  bool
	}{
		{
			name:    "ImagePresent: valid test",
			imgName: options.GetDockerImageId(options.IMG_OVS),
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
			present, err := client.IsImagePresent(context.Background(), tt.imgName)
			if err != nil {
				t.Errorf("IsImagePresent returns an error: %v", err)
			} else if present != tt.result {
				t.Errorf("IsImagePresent returns wrong result: %v != %v", present, tt.result)
			}
		})
	}
}

func TestDockerClient_CreateRm(t *testing.T) {
	client, cID, _, teardown := setupContainer(t, options.IMG_HOST)
	defer teardown()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// check the existence of the container
	container, err := client.Get(ctx, cID)
	if err != nil {
		t.Errorf("Unable to get info about the created container: %v", err)
	} else if container == nil {
		t.Error("The container has not been created")
	}

	// start the container
	if err = client.Start(ctx, cID); err != nil {
		t.Errorf("Unable to start the container: %v", err)
	}

	// check the status of the container
	container, err = client.Get(ctx, cID)
	if err != nil {
		t.Errorf("Unable to get info about the created container: %v", err)
	}
	if container.State != "running" {
		t.Errorf("The container is not started %s != 'running'", container.State)
	}

	// stop the container
	if err = client.Stop(ctx, cID); err != nil {
		t.Errorf("Unable to stop the container: %v", err)
	}
}

func TestDockerClient_Exec(t *testing.T) {
	client, cID, name, teardown := setupStartedContainer(t, options.IMG_SERVER)
	defer teardown()

	// get the hostname
	hostname, err := client.Exec(context.Background(), cID, []string{"hostname"})
	if err != nil {
		t.Fatalf("Unable to get the hostname with exec call: %v", err)
	}
	if strings.TrimRight(hostname, "\n") != name {
		t.Fatalf("The hostname is not correct: %s != %s", hostname, name)
	}
}

func TestDockerClient_Copy(t *testing.T) {
	client, cID, _, teardown := setupContainer(t, options.IMG_ROUTER)
	defer teardown()

	// create a file and copy it in the container
	tmpData := utils.RandString(32)
	tmpFilename := fmt.Sprintf("/tmp/%s.temp", utils.RandString(12))
	err := os.WriteFile(tmpFilename, []byte(tmpData), 0644)
	if err != nil {
		t.Fatalf("Unable to create a temp file: %v", err)
	}
	defer os.Remove(tmpFilename)

	err = client.CopyTo(context.Background(), cID, tmpFilename, "/tmp/test.temp")
	if err != nil {
		t.Fatalf("Unable to copy file to container: %v", err)
	}

	// copy the same file from the container
	newTmpFilename := fmt.Sprintf("/tmp/%s.temp", utils.RandString(12))
	err = client.CopyFrom(context.Background(), cID, "/tmp/test.temp", newTmpFilename)
	if err != nil {
		t.Fatalf("Unable to copy file from the container: %v", err)
	}
	defer os.Remove(newTmpFilename)

	// check the content of the new file
	data, err := os.ReadFile(newTmpFilename)
	if err != nil {
		t.Fatalf("Unable to read the new temp file: %v", err)
	}
	if string(data) != tmpData {
		t.Fatalf("Data are different between CopyTo/CopyFrom: %s != %s", string(data), tmpData)
	}
}

func TestDockerClient_Pid(t *testing.T) {
	client, cID, _, teardown := setupContainer(t, options.IMG_ROUTER)
	defer teardown()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// pid must be equal to -1
	pid, _ := client.Pid(ctx, cID)
	if pid != -1 {
		t.Errorf("Container is not running but Pid != -1, Pid == %d", pid)
	}

	// start container
	if err := client.Start(ctx, cID); err != nil {
		t.Errorf("Unable to start the container: %v", err)
	}

	// pid must be different to -1
	pid, _ = client.Pid(ctx, cID)
	if pid == -1 {
		t.Error("Container is running but Pid == -1", pid)
	}

	// stop
	if err := client.Stop(ctx, cID); err != nil {
		t.Errorf("Unable to stop the container: %v", err)
	}
}

func TestDockerClient_List(t *testing.T) {
	client, _, name, teardown := setupContainer(t, options.IMG_ROUTER)
	defer teardown()

	cList, err := client.List(context.Background(), name)
	if err != nil {
		t.Fatalf("Unable to get container list: %v", err)
	}

	if len(cList) != 1 {
		t.Fatal("Created container notfound in the list")
	}
}
