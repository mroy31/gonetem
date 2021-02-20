package docker

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/mroy31/gonetem/internal/link"
	"github.com/mroy31/gonetem/internal/options"
	"github.com/mroy31/gonetem/internal/utils"
)

func skipUnlessRoot(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Test requires root privileges.")
	}
}

func TestDockerNode_StartStop(t *testing.T) {
	options.InitServerConfig()
	tests := []struct {
		desc        string
		name        string
		nType       string
		ipv6        bool
		mpls        bool
		expectError bool
	}{
		{
			desc:        "DockerNode: random type test",
			name:        utils.RandString(3),
			nType:       utils.RandString(8),
			expectError: true,
		},
		{
			desc:        "DockerNode: host start/stop test",
			name:        utils.RandString(3),
			nType:       "host",
			ipv6:        true,
			mpls:        false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			prjID := utils.RandString(4)
			config := DockerNodeOptions{
				Name: tt.name,
				Type: tt.nType,
				Ipv6: tt.ipv6,
				Mpls: tt.mpls,
			}

			node, err := NewDockerNode(prjID, config)
			if tt.expectError {
				if err == nil {
					t.Errorf("An error is expected but it is not occurred")
				}
				return
			}
			if err != nil {
				t.Errorf("Unable to create docker node: %v", err)
				return
			}

			// Start the node
			if err := node.Start(); err != nil {
				t.Errorf("Unable to start docker node: %v", err)
			}

			// Check the status
			status, err := node.GetStatus()
			if err != nil {
				t.Errorf("Unable to get docker node status: %v", err)
			} else if !status.Running {
				t.Error("The docker node is not running even after start command")
			}

			// Clean up
			node.Close()
		})
	}
}

func TestDockerNode_Copy(t *testing.T) {
	options.InitServerConfig()
	prjID := utils.RandString(4)
	config := DockerNodeOptions{
		Name: utils.RandString(3),
		Type: "router",
	}

	node, err := NewDockerNode(prjID, config)
	if err != nil {
		t.Errorf("Unable to create docker node: %v", err)
		return
	}
	defer node.Close()

	// Test copy from
	localFile := "/tmp/frr-test.conf"
	if err := node.CopyFrom("/etc/frr/frr.conf", localFile); err != nil {
		t.Errorf("Unable to copy a file from docker node: %v", err)
		return
	}
	stat, err := os.Stat(localFile)
	if err != nil || !stat.Mode().IsRegular() {
		t.Errorf("The local file has not bee created")
		return
	}
	defer os.Remove(localFile)

	// Test copy to
	if err := node.CopyTo(localFile, "/tmp/"); err != nil {
		t.Errorf("Unable to copy a file to docker node: %v", err)
		return
	}
}

func TestDockerNode_Save(t *testing.T) {
	options.InitServerConfig()
	prefix := utils.RandString(3)
	tests := []struct {
		desc      string
		name      string
		nType     string
		ipv6      bool
		mpls      bool
		confFiles []string
	}{
		{
			desc:      "DockerNode: save host node",
			name:      fmt.Sprintf("%s-%s", prefix, "host"),
			nType:     "host",
			ipv6:      true,
			mpls:      false,
			confFiles: []string{"%s.net.conf", "%s.ntp.conf"},
		},
		{
			desc:      "DockerNode: save server node",
			name:      fmt.Sprintf("%s-%s", prefix, "server"),
			nType:     "server",
			ipv6:      true,
			mpls:      false,
			confFiles: []string{"%s.net.conf", "%s.dhcpd.conf"},
		},
		{
			desc:      "DockerNode: save router node",
			name:      fmt.Sprintf("%s-%s", prefix, "router"),
			nType:     "router",
			ipv6:      false,
			mpls:      true,
			confFiles: []string{"%s.frr.conf"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			prjID := utils.RandString(4)
			config := DockerNodeOptions{
				Name: tt.name,
				Type: tt.nType,
				Ipv6: tt.ipv6,
				Mpls: tt.mpls,
			}

			node, err := NewDockerNode(prjID, config)
			if err != nil {
				t.Errorf("Unable to create docker node: %v", err)
				return
			}
			defer node.Close()

			// Start the node and load config
			if err := node.Start(); err != nil {
				t.Errorf("Unable to start node %s: %v", tt.name, err)
				return
			}
			if err := node.LoadConfig("/tmp/fake"); err != nil {
				t.Errorf("Unable to load config for node %s: %v", tt.name, err)
				return
			}

			// create temp dir to save configuration files
			dir, err := ioutil.TempDir("/tmp", "ntmtst")
			if err != nil {
				t.Errorf("Unable to create temp folder: %v", err)
				return
			}
			defer os.RemoveAll(dir)

			// save configuration
			if err := node.Save(dir); err != nil {
				t.Errorf("Unable to save node %s: %v", tt.name, err)
				return
			}

			// check file
			for _, f := range tt.confFiles {
				fPath := path.Join(dir, fmt.Sprintf(f, tt.name))
				if _, err := os.Stat(fPath); os.IsNotExist(err) {
					t.Errorf("File '%s' is not found", fPath)
				}
			}
		})
	}
}

func TestDockerNode_AttachLink(t *testing.T) {
	skipUnlessRoot(t)

	options.InitServerConfig()
	prjID := utils.RandString(4)
	// Create 2 nodes and create a link between
	config := DockerNodeOptions{
		Name: utils.RandString(3),
		Type: "router",
	}
	node1, err := NewDockerNode(prjID, config)
	if err != nil {
		t.Fatalf("Unable to create docker node: %v", err)
	}
	defer node1.Close()

	config = DockerNodeOptions{
		Name: utils.RandString(3),
		Type: "host",
	}
	node2, err := NewDockerNode(prjID, config)
	if err != nil {
		t.Fatalf("Unable to create docker node: %v", err)
	}
	defer node2.Close()

	for _, node := range []*DockerNode{node1, node2} {
		if err := node.Start(); err != nil {
			t.Fatalf("Unable to start docker node %s: %v", node.GetName(), err)
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
