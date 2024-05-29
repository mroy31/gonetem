package server

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/mroy31/gonetem/internal/options"
	"github.com/mroy31/gonetem/internal/utils"
)

type ProjectNotFoundError struct {
	Id string
}

func (e *ProjectNotFoundError) Error() string {
	return "Project " + e.Id + ": not found"
}

type NetemProject struct {
	Id       string
	Name     string
	Dir      string
	OpenAt   time.Time
	Topology *NetemTopologyManager
}

var (
	openProjects = make(map[string]*NetemProject, 0)
)

func ProjectIsExist(prjName string) bool {
	for _, prj := range openProjects {
		if prj.Name == prjName {
			return true
		}
	}
	return false
}

func ProjectIsIdExist(prjID string) bool {
	for _, prj := range openProjects {
		if prj.Id == prjID {
			return true
		}
	}
	return false
}

func ProjectGetMany() map[string]*NetemProject {
	return openProjects
}

func ProjectGetOne(prjID string) *NetemProject {
	prj, found := openProjects[prjID]
	if found {
		return prj
	}
	return nil
}

func ProjectOpen(prjId, name string, data []byte) (*NetemProject, error) {
	// create temp directory for the project
	dir, err := os.MkdirTemp(options.ServerConfig.Workdir, "gonetem-"+prjId+"-")
	if err != nil {
		return nil, fmt.Errorf("unable to create temp folder for project: %w", err)
	}

	if err := utils.OpenArchive(dir, bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("unable to open project: %w", err)
	}

	// load the topology
	topology, err := LoadTopology(prjId, dir)
	if err != nil {
		defer func() {
			topology.Close(nil)
			os.RemoveAll(dir)
		}()
		return nil, err
	}

	prj := &NetemProject{
		Id:       prjId,
		Name:     name,
		Dir:      dir,
		OpenAt:   time.Now(),
		Topology: topology,
	}
	openProjects[prjId] = prj
	return prj, nil
}

func ProjectSave(prjId string, progressCh chan TopologySaveProgressT) (*bytes.Buffer, error) {
	project := ProjectGetOne(prjId)
	if project == nil {
		return nil, &ProjectNotFoundError{prjId}
	}

	if err := project.Topology.Save(progressCh); err != nil {
		return nil, err
	}

	buffer := new(bytes.Buffer)
	if err := utils.CreateArchive(project.Dir, buffer); err != nil {
		return nil, err
	}
	return buffer, nil
}

func ProjectGetNodeConfigs(prjId string) (*bytes.Buffer, error) {
	project := ProjectGetOne(prjId)
	if project == nil {
		return nil, &ProjectNotFoundError{prjId}
	}

	// save project before return config archive
	if err := project.Topology.Save(nil); err != nil {
		return nil, err
	}

	configPath := path.Join(project.Dir, configDir)
	buffer := new(bytes.Buffer)
	if err := utils.CreateArchive(configPath, buffer); err != nil {
		return nil, err
	}
	return buffer, nil
}

func ProjectClose(prjId string, progressCh chan TopologyRunCloseProgressT) error {
	project := ProjectGetOne(prjId)
	if project == nil {
		return &ProjectNotFoundError{prjId}
	}

	defer os.RemoveAll(project.Dir)
	defer delete(openProjects, prjId)

	return project.Topology.Close(progressCh)
}
