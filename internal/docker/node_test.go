package docker

import (
	"context"
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/mroy31/gonetem/internal/link"
	"github.com/mroy31/gonetem/internal/options"
	"github.com/mroy31/gonetem/internal/utils"
)

const (
	NODE_TIMEOUT_OP = 10
)

func skipUnlessRoot(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Test requires root privileges.")
	}
}

func TestDockerNode_StartStop(t *testing.T) {
	skipUnlessRoot(t)

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
			mpls:        true,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			prjID := utils.RandString(4)
			options := DockerNodeOptions{
				Name: tt.name,
				Ipv6: tt.ipv6,
				Mpls: tt.mpls,
			}

			node, err := NewDockerNode(prjID, tt.nType, options)
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
	skipUnlessRoot(t)

	options.InitServerConfig()
	prjID := utils.RandString(4)
	config := DockerNodeOptions{
		Name: utils.RandString(3),
	}

	node, err := NewDockerNode(prjID, "router", config)
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

func TestDockerNode_ReadConfigFiles(t *testing.T) {
	skipUnlessRoot(t)

	options.InitServerConfig()
	prefix := utils.RandString(3)
	tests := []struct {
		desc          string
		name          string
		nType         string
		confFileNames []string
	}{
		{
			desc:          "DockerNode: read config files of host node",
			name:          fmt.Sprintf("%s-%s", prefix, "host"),
			nType:         "host",
			confFileNames: []string{"Init", "Network", "NTP"},
		},
		{
			desc:          "DockerNode: read config files of server node",
			name:          fmt.Sprintf("%s-%s", prefix, "server"),
			nType:         "server",
			confFileNames: []string{"Init", "Network", "NTP", "DHCP", "TFTP", "DHCP-RELAY", "Bind"},
		},
		{
			desc:          "DockerNode: read config files of router node",
			name:          fmt.Sprintf("%s-%s", prefix, "router"),
			nType:         "router",
			confFileNames: []string{"FRR"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			prjID := utils.RandString(4)
			config := DockerNodeOptions{
				Name: tt.name,
			}

			node, err := NewDockerNode(prjID, tt.nType, config)
			if err != nil {
				t.Fatalf("Unable to create docker node: %v", err)
			}
			defer node.Close()

			// Start the node and load config
			if err := node.Start(); err != nil {
				t.Fatalf("Unable to start node %s: %v", tt.name, err)
			}

			// create temp dir to save configuration files
			dir, err := os.MkdirTemp("/tmp", "ntmtst")
			if err != nil {
				t.Fatalf("Unable to create temp folder: %v", err)
			}
			defer os.RemoveAll(dir)

			// read config files
			configFiles, err := node.ReadConfigFiles(dir, NODE_TIMEOUT_OP)
			if err != nil {
				t.Fatalf("Unable to read config files of %s: %v", tt.name, err)
			}

			if len(configFiles) != len(tt.confFileNames) {
				t.Errorf(
					"Wrong number of config files of %d != %d: %v",
					len(configFiles), len(tt.confFileNames), err)
			}
		})
	}
}

func TestDockerNode_Save(t *testing.T) {
	skipUnlessRoot(t)

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
				Ipv6: tt.ipv6,
				Mpls: tt.mpls,
			}

			node, err := NewDockerNode(prjID, tt.nType, config)
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
			if _, err := node.LoadConfig("/tmp/fake", NODE_TIMEOUT_OP); err != nil {
				t.Errorf("Unable to load config for node %s: %v", tt.name, err)
				return
			}

			// create temp dir to save configuration files
			dir, err := os.MkdirTemp("/tmp", "ntmtst")
			if err != nil {
				t.Errorf("Unable to create temp folder: %v", err)
				return
			}
			defer os.RemoveAll(dir)

			// save configuration
			if err := node.Save(dir, NODE_TIMEOUT_OP); err != nil {
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
	}
	node1, err := NewDockerNode(prjID, "router", config)
	if err != nil {
		t.Fatalf("Unable to create docker node: %v", err)
	}
	defer node1.Close()

	config = DockerNodeOptions{
		Name: utils.RandString(3),
	}
	node2, err := NewDockerNode(prjID, "host", config)
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

func TestDockerNode_Volumes(t *testing.T) {
	skipUnlessRoot(t)

	options.InitServerConfig()
	prjID := utils.RandString(4)

	// Create 1 host node
	config := DockerNodeOptions{
		Name:    utils.RandString(3),
		Volumes: []string{"/tmp:/tmp/volume"},
	}
	node, err := NewDockerNode(prjID, "host", config)
	if err != nil {
		t.Fatalf("Unable to create docker node: %v", err)
	}
	defer node.Close()

	// start node
	if err := node.Start(); err != nil {
		t.Fatalf("Unable to start docker node %s: %v", node.GetName(), err)
	}

	// create empty file in /tmp folder
	filename := fmt.Sprintf("%s.txt", utils.RandString(6))
	target := path.Join("/tmp", filename)
	if err := os.WriteFile(target, []byte{}, 0666); err != nil {
		t.Fatalf("Unable to create temp file %s: %v", target, err)
	}
	defer os.Remove(filename)

	// check this file exist in node
	nTarget := path.Join("/tmp/volume", filename)
	client, err := NewDockerClient()
	if err != nil {
		t.Fatalf("Unable to create docker client: %v", err)
	}
	defer client.Close()

	if !client.IsFileExist(context.Background(), node.ID, nTarget) {
		t.Fatalf("File %s is not present in the binding volume", nTarget)
	}
}
