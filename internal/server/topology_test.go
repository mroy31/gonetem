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
	updateLinkTopo = `
nodes:
  R1:
    type: docker.router
  R2:
    type: docker.router
links:
- peer1: R1.0
  peer2: R2.0
`
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

func TestTopology_UpdateLink(t *testing.T) {
	options.InitServerConfig()
	tests := []struct {
		desc          string
		topology      string
		peer1         string
		peer2         string
		delay         int
		jitter        int
		loss          float64
		expectedError bool
	}{
		{
			desc:     "Topology: update existing link",
			topology: updateLinkTopo,
			peer1:    "R1.0",
			peer2:    "R2.0",
			delay:    50,
			jitter:   10,
			loss:     0.1,
		},
		{
			desc:          "Topology: update wrong link",
			topology:      updateLinkTopo,
			peer1:         "R1.0",
			peer2:         "R3.0",
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
			if err != nil {
				t.Errorf("LoadTopology returns an unexpected error: %v", err)
				return
			}
			defer topology.Close(nil)

			if _, err := topology.Run(nil); err != nil {
				t.Errorf("Run returns an error: %v", err)
				return
			}

			if err := topology.LinkUpdate(LinkConfig{
				Peer1:  tt.peer1,
				Peer2:  tt.peer2,
				Delay:  tt.delay,
				Jitter: tt.jitter,
				Loss:   tt.loss,
			}, true); err != nil && !tt.expectedError {
				t.Errorf("LinkUpdate returns an unexpected error: %v", err)
				return
			}

			if !tt.expectedError {
				link, _, _ := topology.GetLink(tt.peer1, tt.peer2)
				if link.Config.Delay != tt.delay || link.Config.Jitter != tt.jitter {
					t.Errorf(
						"Delay or jitter have wrong value: %d|%d %d|%d",
						link.Config.Delay, tt.delay, link.Config.Jitter, tt.jitter,
					)
				}
			}
		})
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
				topology.Close(nil)
				return
			}
			defer topology.Close(nil)

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
	defer topology.Close(nil)

	// start all nodes and save configuration
	if _, err := topology.Run(nil); err != nil {
		t.Errorf("Run returns an error: %v", err)
	}
	if err := topology.Save(nil); err != nil {
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
