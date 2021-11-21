package server

import (
	"bytes"
	"fmt"
	"io/ioutil"
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

func IsProjectExist(prjName string) bool {
	for _, prj := range openProjects {
		if prj.Name == prjName {
			return true
		}
	}
	return false
}

func IdProjectExist(prjID string) bool {
	for _, prj := range openProjects {
		if prj.Id == prjID {
			return true
		}
	}
	return false
}

func GetAllProjects() map[string]*NetemProject {
	return openProjects
}

func GetProject(prjID string) *NetemProject {
	prj, found := openProjects[prjID]
	if found {
		return prj
	}
	return nil
}

func OpenProject(prjId, name string, data []byte) (*NetemProject, error) {
	// create temp directory for the project
	dir, err := ioutil.TempDir(options.ServerConfig.Workdir, "gonetem-"+prjId+"-")
	if err != nil {
		return nil, fmt.Errorf("Unable to create temp folder for project: %w", err)
	}

	if err := utils.OpenArchive(dir, bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("Unable to open project: %w", err)
	}

	// load the topology
	topology, err := LoadTopology(prjId, dir)
	if err != nil {
		defer func() {
			topology.Close()
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

func SaveProject(prjId string) (*bytes.Buffer, error) {
	project := GetProject(prjId)
	if project == nil {
		return nil, &ProjectNotFoundError{prjId}
	}

	if err := project.Topology.Save(); err != nil {
		return nil, err
	}

	buffer := new(bytes.Buffer)
	if err := utils.CreateArchive(project.Dir, buffer); err != nil {
		return nil, err
	}
	return buffer, nil
}

func GetProjectConfigs(prjId string) (*bytes.Buffer, error) {
	project := GetProject(prjId)
	if project == nil {
		return nil, &ProjectNotFoundError{prjId}
	}

	// save project before return config archive
	if err := project.Topology.Save(); err != nil {
		return nil, err
	}

	configPath := path.Join(project.Dir, configDir)
	buffer := new(bytes.Buffer)
	if err := utils.CreateArchive(configPath, buffer); err != nil {
		return nil, err
	}
	return buffer, nil
}

func CloseProject(prjId string) error {
	project := GetProject(prjId)
	if project == nil {
		return &ProjectNotFoundError{prjId}
	}

	defer os.RemoveAll(project.Dir)
	defer delete(openProjects, prjId)

	return project.Topology.Close()
}
