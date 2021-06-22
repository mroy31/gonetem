package server

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/mroy31/gonetem/internal/options"
	"github.com/mroy31/gonetem/internal/utils"
)

func createProject(filepath string, topo TopologyTestData) error {
	prj, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer prj.Close()

	return utils.CreateOneFileArchive(prj, networkFilename, []byte(topo.network))
}

func TestProject_OpenClose(t *testing.T) {
	options.InitServerConfig()

	prjPath := "/tmp/prjtest-archive.gnet"
	if err := createProject(prjPath, simpleNetwork); err != nil {
		t.Fatalf("Unable to create .gnet file: %v", err)
	}
	defer os.Remove(prjPath)

	data, err := ioutil.ReadFile(prjPath)
	if err != nil {
		t.Fatalf("Unable to open created .gnet file: %v", err)
	}

	// open project
	prjID := utils.RandString(4)
	project, err := OpenProject(prjID, "PrjTest", data)
	if err != nil {
		t.Fatalf("Unable to open project: %v", err)
	}
	defer CloseProject(prjID)

	// check topology
	checkTopology(simpleNetwork, project.Topology, t)
}

func TestProject_Save(t *testing.T) {
	options.InitServerConfig()

	prjPath := "/tmp/prjtest-archive.gnet"
	if err := createProject(prjPath, simpleNetwork); err != nil {
		t.Errorf("Unable to create .gnet file: %v", err)
		return
	}
	defer os.Remove(prjPath)

	data, err := ioutil.ReadFile(prjPath)
	if err != nil {
		t.Errorf("Unable to open created .gnet file: %v", err)
		return
	}

	// open project
	prjID := utils.RandString(4)
	project, err := OpenProject(prjID, "PrjTest", data)
	if err != nil {
		t.Errorf("Unable to open project: %v", err)
		return
	}
	defer CloseProject(prjID)

	if _, err := project.Topology.Run(); err != nil {
		t.Errorf("Unable to start project: %v", err)
		return
	}

	if err := project.Topology.Save(); err != nil {
		t.Errorf("Unable to save project: %v", err)
		return
	}

	for _, n := range simpleNetwork.nodes {
		node := project.Topology.GetNode(n.name)
		if node == nil {
			t.Errorf("Node %s is not found", n)
		} else if !node.IsRunning() {
			t.Errorf("Node %s is not running", n)
		}
	}
}
