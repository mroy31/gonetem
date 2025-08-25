package options

import (
	"fmt"
	"testing"
)

type DockerImageT int

const (
	IMG_ROUTER DockerImageT = iota
	IMG_HOST
	IMG_SERVER
	IMG_OVS
)

func getImageFromT(imgId DockerImageT) string {
	switch imgId {
	case IMG_ROUTER:
		return GetDockerImageId(ServerConfig.Docker.Nodes.Router.Image)
	case IMG_HOST:
		return GetDockerImageId(ServerConfig.Docker.Nodes.Host.Image)
	case IMG_SERVER:
		return GetDockerImageId(ServerConfig.Docker.Nodes.Server.Image)
	case IMG_OVS:
		return GetDockerImageId(ServerConfig.Docker.OvsImage)
	}

	return ""
}

func TestOptions_ImageId(t *testing.T) {
	InitServerConfig()

	tests := []struct {
		imgType    DockerImageT
		expectedID string
	}{
		{
			imgType:    IMG_HOST,
			expectedID: fmt.Sprintf("%s:%s", ServerConfig.Docker.Nodes.Host.Image, VERSION),
		},
		{
			imgType:    IMG_SERVER,
			expectedID: fmt.Sprintf("%s:%s", ServerConfig.Docker.Nodes.Server.Image, VERSION),
		},
		{
			imgType:    IMG_ROUTER,
			expectedID: fmt.Sprintf("%s:%s", ServerConfig.Docker.Nodes.Router.Image, VERSION),
		},
		{
			imgType:    IMG_OVS,
			expectedID: fmt.Sprintf("%s:%s", ServerConfig.Docker.OvsImage, VERSION),
		},
	}
	for _, test := range tests {
		id := GetDockerImageId(getImageFromT(test.imgType))
		if id != test.expectedID {
			t.Errorf("Error: %s != %s", id, test.expectedID)
		}
	}

	id := GetDockerImageId("mroy31/ovs-img:0.0.0")
	if id != "mroy31/ovs-img:0.0.0" {
		t.Fatalf("Error: %s != mroy31/ovs-img:0.0.0", id)
	}
}
