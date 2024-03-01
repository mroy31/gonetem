package server

import (
	"os"
	"path"
	"testing"

	"github.com/mroy31/gonetem/internal/options"
	"github.com/mroy31/gonetem/internal/utils"
)

type TopologyTestData struct {
	network string
	nodes   []struct {
		name   string
		kind   string
		launch bool
	}
}

var (
	simpleNetwork = TopologyTestData{
		network: `
nodes:
  switch:
    type: ovs
  R1:
    type: docker.router
    ipv6: true
    mpls: true
  host:
    type: docker.host
  hostNotLaunch:
    type: docker.host
    launch: false
links:
- peer1: R1.0
  peer2: switch.0
- peer1: host.0
  peer2: switch.1
- peer1: hostNotLaunch.0
  peer2: switch.2`,
		nodes: []struct {
			name   string
			kind   string
			launch bool
		}{
			{
				name:   "switch",
				kind:   "ovs",
				launch: true,
			},
			{
				name:   "R1",
				kind:   "docker",
				launch: true,
			},
			{
				name:   "host",
				kind:   "docker",
				launch: true,
			},
			{
				name:   "hostNotLaunch",
				kind:   "docker",
				launch: false,
			},
		},
	}
	wrongTopology = `
nodes:
  R1
    type: docker.router
    ipv6: true
    mpls: true
  host
    type: test
`
)

func checkTopology(data TopologyTestData, topology *NetemTopologyManager, t *testing.T) {
	for _, n := range data.nodes {
		node := topology.GetNode(n.name)
		if node == nil {
			t.Errorf("Node %s is not found", n.name)
		} else {
			if node.GetName() != n.name {
				t.Errorf("Node has wrong name %s != %s", n.name, node.GetName())
			}
			if node.GetType() != n.kind {
				t.Errorf("Node %s has wrong type %s != %s", n.name, n.kind, node.GetType())
			}
			if topology.IsNodeLaunchAtStartup(n.name) != n.launch {
				t.Errorf("Node %s has wrong launch argument", n.name)
			}
		}
	}

}

func TestTopology_Load(t *testing.T) {
	options.InitServerConfig()
	tests := []struct {
		desc          string
		topology      string
		nodes         []string
		expectedError bool
	}{
		{
			desc:     "Topology: load simple topology",
			topology: simpleNetwork.network,
			nodes:    []string{"R1", "host"},
		},
		{
			desc:          "Topology: load bad topology",
			topology:      wrongTopology,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			prjID := utils.RandString(4)
			// create temp dir to save configuration files
			dir, err := os.MkdirTemp("/tmp", "ntmtst")
			if err != nil {
				t.Errorf("Unable to create temp folder: %v", err)
				return
			}
			defer os.RemoveAll(dir)

			if err := os.WriteFile(path.Join(dir, "network.yml"), []byte(tt.topology), 0644); err != nil {
				t.Errorf("Unable to create topology file: %v", err)
				return
			}

			topology, err := LoadTopology(prjID, dir)
			if err != nil && !tt.expectedError {
				t.Errorf("LoadTopology returns an unexpected error: %v", err)
			} else if tt.expectedError {
				topology.Close()
				return
			}
			defer topology.Close()

			for _, n := range tt.nodes {
				node := topology.GetNode(n)
				if node == nil {
					t.Errorf("Node %s is not found", n)
				} else if node.GetName() != n {
					t.Errorf("Node has wrong name %s != %s", n, node.GetName())
				}
			}
		})
	}
}

func TestTopology_Save(t *testing.T) {
	prjID := utils.RandString(4)
	// create temp dir to save configuration files
	dir, err := os.MkdirTemp("/tmp", "ntmtst")
	if err != nil {
		t.Errorf("Unable to create temp folder: %v", err)
		return
	}
	defer os.RemoveAll(dir)

	if err := os.WriteFile(path.Join(dir, "network.yml"), []byte(simpleNetwork.network), 0644); err != nil {
		t.Errorf("Unable to create topology file: %v", err)
		return
	}

	topology, err := LoadTopology(prjID, dir)
	if err != nil {
		t.Errorf("LoadTopology returns an unexpected error: %v", err)
	}
	defer topology.Close()

	// start all nodes and save configuration
	if _, err := topology.Run(); err != nil {
		t.Errorf("Run returns an error: %v", err)
	}
	if err := topology.Save(); err != nil {
		t.Errorf("Save returns an error: %v", err)
	}

	// check config files
	files := []string{"host.net.conf", "R1.frr.conf"}
	for _, f := range files {
		filePath := path.Join(dir, "configs", f)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Config file '%s' has not been created", f)
		}
	}
}
