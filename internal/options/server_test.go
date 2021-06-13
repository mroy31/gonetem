package options

import (
	"fmt"
	"testing"
)

func TestOptions_ImageId(t *testing.T) {
	InitServerConfig()

	tests := []struct {
		imgType    DockerImageT
		expectedID string
	}{
		{
			imgType:    IMG_HOST,
			expectedID: fmt.Sprintf("%s:%s", ServerConfig.Docker.Images.Host, IMG_VERSION),
		},
		{
			imgType:    IMG_SERVER,
			expectedID: fmt.Sprintf("%s:%s", ServerConfig.Docker.Images.Server, IMG_VERSION),
		},
		{
			imgType:    IMG_ROUTER,
			expectedID: fmt.Sprintf("%s:%s", ServerConfig.Docker.Images.Router, IMG_VERSION),
		},
		{
			imgType:    IMG_OVS,
			expectedID: fmt.Sprintf("%s:%s", ServerConfig.Docker.Images.Ovs, IMG_VERSION),
		},
	}
	for _, test := range tests {
		id := GetDockerImageId(test.imgType)
		if id != test.expectedID {
			t.Errorf("Error: %s != %s", id, test.expectedID)
		}
	}

	ServerConfig.Docker.Images.Ovs = "mroy31/ovs-img:0.0.0"
	id := GetDockerImageId(IMG_OVS)
	if id != "mroy31/ovs-img:0.0.0" {
		t.Fatalf("Error: %s != mroy31/ovs-img:0.0.0", id)
	}
}
