package server

import (
	context "context"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/moby/term"
	"github.com/mroy31/gonetem/internal/docker"
	"github.com/mroy31/gonetem/internal/link"
	"github.com/mroy31/gonetem/internal/options"
	"github.com/mroy31/gonetem/internal/proto"
	"github.com/mroy31/gonetem/internal/utils"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type netemServer struct {
	proto.UnimplementedNetemServer
}

func (s *netemServer) GetVersion(ctx context.Context, empty *empty.Empty) (*proto.VersionResponse, error) {
	return &proto.VersionResponse{
		Status: &proto.Status{
			Code: proto.StatusCode_OK,
		},
		Version: options.VERSION,
	}, nil
}

func (s *netemServer) Clean(ctx context.Context, empty *empty.Empty) (*proto.AckResponse, error) {
	client, err := docker.NewDockerClient()
	if err != nil {
		return nil, fmt.Errorf("unable to init docker client: %w", err)
	}
	defer client.Close()

	cList, err := client.List(options.NETEM_ID)
	if err != nil {
		return nil, fmt.Errorf("unable to get container list: %w", err)
	}

	re := regexp.MustCompile(`^` + options.NETEM_ID + `(\w+)\.\w+`)
	for _, cObj := range cList {
		groups := re.FindStringSubmatch(cObj.Name)
		if len(groups) == 2 {
			if !IdProjectExist(groups[1]) {
				logrus.Infof("Clean: remove container %s\n", cObj.Name)
				if err := client.Stop(cObj.Container.ID); err != nil {
					return nil, fmt.Errorf("unable to stop container %s: %w", cObj.Name, err)
				}
				if err := client.Rm(cObj.Container.ID); err != nil {
					return nil, fmt.Errorf("unable to rm container %s: %w", cObj.Name, err)
				}
			}
		}
	}

	return &proto.AckResponse{
		Status: &proto.Status{Code: proto.StatusCode_OK},
	}, nil
}

func (s *netemServer) PullImages(empty *empty.Empty, stream proto.Netem_PullImagesServer) error {
	imageTypes := []options.DockerImageT{
		options.IMG_HOST,
		options.IMG_OVS,
		options.IMG_ROUTER,
		options.IMG_SERVER,
	}

	client, err := docker.NewDockerClient()
	if err != nil {
		return err
	}
	defer client.Close()

	for _, imgT := range imageTypes {
		imgID := options.GetDockerImageId(imgT)

		stream.Send(&proto.PullSrvMsg{
			Code:  proto.PullSrvMsg_START,
			Image: imgID,
		})

		if err := client.ImagePull(imgID); err != nil {
			stream.Send(&proto.PullSrvMsg{
				Code:  proto.PullSrvMsg_ERROR,
				Image: imgID,
				Error: fmt.Sprintf("%s: %v", imgID, err),
			})
			continue
		}
		stream.Send(&proto.PullSrvMsg{
			Code:  proto.PullSrvMsg_OK,
			Image: imgID,
		})
	}

	return nil
}

func (s *netemServer) GetProjects(ctx context.Context, empty *empty.Empty) (*proto.PrjListResponse, error) {
	prjList := GetAllProjects()
	response := proto.PrjListResponse{
		Status: &proto.Status{
			Code: proto.StatusCode_OK,
		},
		Projects: make([]*proto.PrjListResponse_Info, 0, len(prjList)),
	}

	for _, prj := range prjList {
		response.Projects = append(response.Projects, &proto.PrjListResponse_Info{
			Id:     prj.Id,
			Name:   prj.Name,
			OpenAt: prj.OpenAt.Format("2006-01-02 15:04:05"),
		})
	}

	return &response, nil
}

func (s *netemServer) OpenProject(ctx context.Context, request *proto.OpenRequest) (*proto.PrjOpenResponse, error) {
	if IsProjectExist(request.GetName()) {
		return &proto.PrjOpenResponse{
			Status: &proto.Status{
				Code:  proto.StatusCode_ERROR,
				Error: "A project with this name is already open.\nUse --name option if you want to open it anyway",
			},
		}, nil
	}

	prjID := utils.RandString(3)
	for IdProjectExist(prjID) {
		prjID = utils.RandString(3)
	}

	prj, err := OpenProject(prjID, request.GetName(), request.GetData())
	if err != nil {
		return nil, err
	}

	return &proto.PrjOpenResponse{
		Status: &proto.Status{Code: proto.StatusCode_OK},
		Id:     prj.Id,
	}, nil
}

func (s *netemServer) CloseProject(ctx context.Context, request *proto.ProjectRequest) (*proto.AckResponse, error) {
	if err := CloseProject(request.GetId()); err != nil {
		return nil, err
	}
	return &proto.AckResponse{
		Status: &proto.Status{Code: proto.StatusCode_OK},
	}, nil
}

func (s *netemServer) SaveProject(ctx context.Context, request *proto.ProjectRequest) (*proto.FileResponse, error) {
	data, err := SaveProject(request.GetId())
	if err != nil {
		return nil, err
	}

	return &proto.FileResponse{
		Status: &proto.Status{Code: proto.StatusCode_OK},
		Data:   data.Bytes(),
	}, nil
}

func (s *netemServer) GetProjectConfigs(ctx context.Context, request *proto.ProjectRequest) (*proto.FileResponse, error) {
	data, err := GetProjectConfigs(request.GetId())
	if err != nil {
		return nil, err
	}

	return &proto.FileResponse{
		Status: &proto.Status{Code: proto.StatusCode_OK},
		Data:   data.Bytes(),
	}, nil
}

func (s *netemServer) GetProjectStatus(ctx context.Context, request *proto.ProjectRequest) (*proto.StatusResponse, error) {
	project := GetProject(request.GetId())
	if project == nil {
		return nil, &ProjectNotFoundError{request.GetId()}
	}

	response := &proto.StatusResponse{
		Status:  &proto.Status{Code: proto.StatusCode_OK},
		Name:    project.Name,
		Id:      project.Id,
		OpenAt:  project.OpenAt.Format("2006-01-02 15:04:05"),
		Running: project.Topology.IsRunning(),
	}

	for _, node := range project.Topology.GetAllNodes() {
		nodeStatus := &proto.StatusResponse_NodeStatus{
			Name:    node.GetName(),
			Running: node.IsRunning(),
		}
		if project.Topology.IsRunning() {
			for ifName, state := range node.GetInterfacesState() {
				nodeStatus.Interfaces = append(nodeStatus.Interfaces, &proto.StatusResponse_IfStatus{
					Name: ifName,
					State: map[link.IfState]proto.IfState{
						link.IFSTATE_DOWN: proto.IfState_DOWN,
						link.IFSTATE_UP:   proto.IfState_UP,
					}[state],
				})
			}
		}

		response.Nodes = append(response.Nodes, nodeStatus)
	}

	return response, nil
}

func (s *netemServer) ReadNetworkFile(ctx context.Context, request *proto.ProjectRequest) (*proto.FileResponse, error) {
	project := GetProject(request.GetId())
	if project == nil {
		return nil, &ProjectNotFoundError{request.GetId()}
	}

	data, err := project.Topology.ReadNetworkFile()
	if err != nil {
		return nil, err
	}
	return &proto.FileResponse{
		Status: &proto.Status{Code: proto.StatusCode_OK},
		Data:   data,
	}, nil
}

func (s *netemServer) WriteNetworkFile(ctx context.Context, request *proto.WNetworkRequest) (*proto.AckResponse, error) {
	project := GetProject(request.GetId())
	if project == nil {
		return nil, &ProjectNotFoundError{request.GetId()}
	}

	if err := project.Topology.WriteNetworkFile(request.GetData()); err != nil {
		return nil, err
	}

	return &proto.AckResponse{
		Status: &proto.Status{Code: proto.StatusCode_OK},
	}, nil
}

func (s *netemServer) LinkUpdate(ctx context.Context, request *proto.LinkRequest) (*proto.AckResponse, error) {
	project := GetProject(request.GetPrjId())
	if project == nil {
		return nil, &ProjectNotFoundError{request.GetPrjId()}
	}

	rLink := request.GetLink()
	linkConfig := LinkConfig{
		Peer1:  rLink.GetPeer1(),
		Peer2:  rLink.GetPeer2(),
		Loss:   float64(rLink.GetLoss()),
		Delay:  int(rLink.GetDelay()),
		Jitter: int(rLink.GetJitter()),
	}

	if err := project.Topology.LinkUpdate(linkConfig); err != nil {
		return nil, err
	}

	return &proto.AckResponse{
		Status: &proto.Status{Code: proto.StatusCode_OK},
	}, nil
}

func (s *netemServer) Run(ctx context.Context, request *proto.ProjectRequest) (*proto.RunResponse, error) {
	project := GetProject(request.GetId())
	if project == nil {
		return nil, &ProjectNotFoundError{request.GetId()}
	}

	nodeMessages, err := project.Topology.Run()
	if err != nil {
		return nil, err
	}

	return &proto.RunResponse{
		Status:       &proto.Status{Code: proto.StatusCode_OK},
		NodeMessages: nodeMessages,
	}, nil
}

func (s *netemServer) Reload(ctx context.Context, request *proto.ProjectRequest) (*proto.RunResponse, error) {
	project := GetProject(request.GetId())
	if project == nil {
		return nil, &ProjectNotFoundError{request.GetId()}
	}

	nodeMessages, err := project.Topology.Reload()
	if err != nil {
		return nil, err
	}

	return &proto.RunResponse{
		Status:       &proto.Status{Code: proto.StatusCode_OK},
		NodeMessages: nodeMessages,
	}, nil
}

func (s *netemServer) Start(ctx context.Context, request *proto.NodeRequest) (*proto.AckResponse, error) {
	project := GetProject(request.GetPrjId())
	if project == nil {
		return nil, &ProjectNotFoundError{request.GetPrjId()}
	}

	if _, err := project.Topology.Start(request.GetNode()); err != nil {
		return nil, err
	}

	return &proto.AckResponse{
		Status: &proto.Status{Code: proto.StatusCode_OK},
	}, nil
}

func (s *netemServer) Stop(ctx context.Context, request *proto.NodeRequest) (*proto.AckResponse, error) {
	project := GetProject(request.GetPrjId())
	if project == nil {
		return nil, &ProjectNotFoundError{request.GetPrjId()}
	}

	if err := project.Topology.Stop(request.GetNode()); err != nil {
		return nil, err
	}

	return &proto.AckResponse{
		Status: &proto.Status{Code: proto.StatusCode_OK},
	}, nil
}

func (s *netemServer) Restart(ctx context.Context, request *proto.NodeRequest) (*proto.AckResponse, error) {
	project := GetProject(request.GetPrjId())
	if project == nil {
		return nil, &ProjectNotFoundError{request.GetPrjId()}
	}

	if err := project.Topology.Stop(request.GetNode()); err != nil {
		return nil, err
	}
	if _, err := project.Topology.Start(request.GetNode()); err != nil {
		return nil, err
	}

	return &proto.AckResponse{
		Status: &proto.Status{Code: proto.StatusCode_OK},
	}, nil
}

func (s *netemServer) Check(ctx context.Context, request *proto.ProjectRequest) (*proto.AckResponse, error) {
	project := GetProject(request.GetId())
	if project == nil {
		return nil, &ProjectNotFoundError{request.GetId()}
	}

	if err := project.Topology.Check(); err != nil {
		return &proto.AckResponse{
			Status: &proto.Status{
				Code:  proto.StatusCode_ERROR,
				Error: err.Error(),
			},
		}, nil
	}

	return &proto.AckResponse{
		Status: &proto.Status{Code: proto.StatusCode_OK},
	}, nil
}

func (s *netemServer) SetIfState(ctx context.Context, request *proto.NodeIfStateRequest) (*proto.AckResponse, error) {
	project := GetProject(request.GetPrjId())
	if project == nil {
		return nil, &ProjectNotFoundError{request.GetPrjId()}
	}

	node := project.Topology.GetNode(request.GetNode())
	if node == nil {
		return nil, &NodeNotFoundError{request.GetPrjId(), request.GetNode()}
	}

	var state link.IfState
	switch request.GetState() {
	case proto.IfState_DOWN:
		state = link.IFSTATE_DOWN
	case proto.IfState_UP:
		state = link.IFSTATE_UP
	}

	if err := node.SetInterfaceState(int(request.GetIfIndex()), state); err != nil {
		return nil, err
	}

	return &proto.AckResponse{
		Status: &proto.Status{Code: proto.StatusCode_OK},
	}, nil
}

func (s *netemServer) Capture(request *proto.NodeInterfaceRequest, stream proto.Netem_CaptureServer) error {
	project := GetProject(request.GetPrjId())
	if project == nil {
		return stream.Send(&proto.CaptureSrvMsg{
			Code: proto.CaptureSrvMsg_ERROR,
			Data: []byte(fmt.Sprintf("Project %s not found", request.PrjId)),
		})
	}

	node := project.Topology.GetNode(request.GetNode())
	if node == nil {
		return stream.Send(&proto.CaptureSrvMsg{
			Code: proto.CaptureSrvMsg_ERROR,
			Data: []byte(fmt.Sprintf("Node %s not found", request.GetNode())),
		})
	}

	stream.Send(&proto.CaptureSrvMsg{
		Code: proto.CaptureSrvMsg_OK,
	})

	logger := logrus.WithFields(logrus.Fields{
		"project": project.Id,
		"node":    node.GetName(),
	})

	rOut, wOut := io.Pipe()
	waitCh := make(chan error)

	go func() {
		defer wOut.Close()

		data := make([]byte, 256)
		for {
			n, err := rOut.Read(data)
			if err != nil {
				if err == io.EOF {
					waitCh <- nil
				} else {
					waitCh <- err
				}
				return
			} else if n == 0 {
				continue
			}

			if err := stream.Send(&proto.CaptureSrvMsg{
				Code: proto.CaptureSrvMsg_STDOUT,
				Data: data[:n],
			}); err != nil {
				waitCh <- err
				return
			}
		}
	}()

	logger.Debugf("Start capture on interface %d", request.IfIndex)
	if err := node.Capture(int(request.GetIfIndex()), wOut); err != nil {
		stream.Send(&proto.CaptureSrvMsg{
			Code: proto.CaptureSrvMsg_ERROR,
			Data: []byte(err.Error()),
		})
	}

	logger.Debugf("Stop capture on interface %d", request.IfIndex)
	wOut.Close()
	return <-waitCh
}

func (s *netemServer) ReadConfigFiles(ctx context.Context, request *proto.NodeRequest) (*proto.ConfigFilesResponse, error) {
	project := GetProject(request.GetPrjId())
	if project == nil {
		return nil, &ProjectNotFoundError{request.GetPrjId()}
	}

	configFiles, err := project.Topology.ReadConfigFiles(request.GetNode())
	if err != nil {
		return &proto.ConfigFilesResponse{
			Status: &proto.Status{
				Code:  proto.StatusCode_ERROR,
				Error: fmt.Sprintf("Unable to read config files of node %s: %v", request.GetNode(), err),
			},
		}, nil
	}

	answer := &proto.ConfigFilesResponse{
		Status: &proto.Status{
			Code: proto.StatusCode_OK,
		},
		Source: proto.ConfigFilesResponse_ARCHIVE,
	}
	if project.Topology.GetNode(request.GetNode()).IsRunning() {
		answer.Source = proto.ConfigFilesResponse_RUNNING
	}

	for name, data := range configFiles {
		answer.Files = append(answer.Files, &proto.ConfigFilesResponse_ConfigFile{
			Name: name,
			Data: data,
		})
	}

	return answer, nil
}

func (s *netemServer) CanRunConsole(ctx context.Context, request *proto.NodeRequest) (*proto.AckResponse, error) {
	project := GetProject(request.GetPrjId())
	if project == nil {
		return nil, &ProjectNotFoundError{request.GetPrjId()}
	}

	node := project.Topology.GetNode(request.GetNode())
	if node == nil {
		return &proto.AckResponse{
			Status: &proto.Status{
				Code:  proto.StatusCode_ERROR,
				Error: fmt.Sprintf("Node %s not found", request.GetNode()),
			},
		}, nil
	}

	if err := node.CanRunConsole(); err != nil {
		return &proto.AckResponse{
			Status: &proto.Status{
				Code:  proto.StatusCode_ERROR,
				Error: err.Error(),
			},
		}, nil
	}

	return &proto.AckResponse{
		Status: &proto.Status{Code: proto.StatusCode_OK},
	}, nil
}

func (s *netemServer) Console(stream proto.Netem_ConsoleServer) error {
	// read first msg from client
	resp, err := stream.Recv()
	if err != nil {
		return err
	}

	// get project
	project := GetProject(resp.GetPrjId())
	if project == nil {
		return stream.Send(&proto.ConsoleSrvMsg{
			Code: proto.ConsoleSrvMsg_ERROR,
			Data: []byte(fmt.Sprintf("Project %s not found", resp.PrjId)),
		})
	}

	// get node
	node := project.Topology.GetNode(resp.GetNode())
	if node == nil {
		return stream.Send(&proto.ConsoleSrvMsg{
			Code: proto.ConsoleSrvMsg_ERROR,
			Data: []byte(fmt.Sprintf("Node %s not found in project %s", resp.GetNode(), resp.GetPrjId())),
		})
	}

	logger := logrus.WithFields(logrus.Fields{
		"project": project.Id,
		"node":    node.GetName(),
	})
	logger.Debug("Start console stream")

	rIn, wIn := io.Pipe()
	rOut, wOut := io.Pipe()

	resizeCh := make(chan term.Winsize)

	g := new(errgroup.Group)
	g.Go(func() error {
		defer close(resizeCh)

		for {
			in, err := stream.Recv()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}

			switch in.GetCode() {
			case proto.ConsoleCltMsg_DATA:
				wIn.Write(in.GetData())
			case proto.ConsoleCltMsg_RESIZE:
				resizeCh <- term.Winsize{
					Width:  uint16(in.GetTtyWidth()),
					Height: uint16(in.GetTtyHeight()),
				}
			case proto.ConsoleCltMsg_CLOSE:
				// TODO: find a solution to terminate shell
				// For now we block terminal close
			}
		}
	})

	g.Go(func() error {
		data := make([]byte, 32)
		for {
			n, err := rOut.Read(data)
			if err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			} else if n == 0 {
				continue
			}

			if err := stream.Send(&proto.ConsoleSrvMsg{
				Code: proto.ConsoleSrvMsg_STDOUT,
				Data: data[:n],
			}); err != nil {
				return err
			}
		}
	})

	if err := node.Console(resp.GetShell(), rIn, wOut, resizeCh); err != nil {
		stream.Send(&proto.ConsoleSrvMsg{
			Code: proto.ConsoleSrvMsg_ERROR,
			Data: []byte(err.Error()),
		})
	} else {
		stream.Send(&proto.ConsoleSrvMsg{
			Code: proto.ConsoleSrvMsg_CLOSE,
		})
	}

	wOut.Close()
	defer wIn.Close()

	logger.Debug("Close console stream")
	return g.Wait()
}

func (s *netemServer) CopyFrom(request *proto.CopyMsg, stream proto.Netem_CopyFromServer) error {
	// get project
	project := GetProject(request.GetPrjId())
	if project == nil {
		return stream.Send(&proto.CopyMsg{
			Code: proto.CopyMsg_ERROR,
			Data: []byte(fmt.Sprintf("Project %s not found", request.PrjId)),
		})
	}

	// get node
	node := project.Topology.GetNode(request.GetNode())
	if node == nil {
		return stream.Send(&proto.CopyMsg{
			Code: proto.CopyMsg_ERROR,
			Data: []byte(fmt.Sprintf("Node %s not found in project %s", request.GetNode(), request.GetPrjId())),
		})
	}

	// first copy file in a temp path
	tempPath := path.Join(
		options.ServerConfig.Workdir,
		fmt.Sprintf(
			"%s-%s-%s", request.GetPrjId(), request.GetNode(),
			path.Base(request.GetNodePath()),
		),
	)
	if err := node.CopyFrom(request.GetNodePath(), tempPath); err != nil {
		return err
	}
	defer os.Remove(tempPath)

	buffer := make([]byte, 1024)
	tempFile, _ := os.Open(tempPath)
	for {
		n, err := tempFile.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if err := stream.Send(&proto.CopyMsg{
			Code: proto.CopyMsg_DATA,
			Data: buffer[:n],
		}); err != nil {
			return err
		}
	}

	return nil
}

func (s *netemServer) CopyTo(stream proto.Netem_CopyToServer) error {
	var tempPath, destPath string
	var tempFile *os.File
	var node INetemNode

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			if tempPath == "" {
				return stream.SendAndClose(&proto.AckResponse{
					Status: &proto.Status{
						Code:  proto.StatusCode_ERROR,
						Error: "Temp file does not exist",
					},
				})
			}
			if err := node.CopyTo(tempPath, destPath); err != nil {
				return stream.SendAndClose(&proto.AckResponse{
					Status: &proto.Status{
						Code:  proto.StatusCode_ERROR,
						Error: err.Error(),
					},
				})
			}

			return stream.SendAndClose(&proto.AckResponse{
				Status: &proto.Status{
					Code: proto.StatusCode_OK,
				},
			})
		}

		switch msg.Code {
		case proto.CopyMsg_INIT:
			project := GetProject(msg.GetPrjId())
			if project == nil {
				return stream.SendAndClose(&proto.AckResponse{
					Status: &proto.Status{
						Code:  proto.StatusCode_ERROR,
						Error: "Project not found",
					},
				})
			}

			// get node
			node = project.Topology.GetNode(msg.GetNode())
			if node == nil {
				return stream.SendAndClose(&proto.AckResponse{
					Status: &proto.Status{
						Code:  proto.StatusCode_ERROR,
						Error: fmt.Sprintf("Node %s not found", msg.GetNode()),
					},
				})
			}

			destPath = msg.GetNodePath()
			tempPath = path.Join(
				options.ServerConfig.Workdir,
				fmt.Sprintf(
					"%s-%s-%s", msg.GetPrjId(), msg.GetNode(),
					path.Base(msg.GetNodePath()),
				),
			)
			tempFile, err = os.Create(tempPath)
			if err != nil {
				tempPath = ""
				return stream.SendAndClose(&proto.AckResponse{
					Status: &proto.Status{
						Code:  proto.StatusCode_ERROR,
						Error: fmt.Sprintf("Unable to create temp file: %v", err),
					},
				})
			}
			defer tempFile.Close()
			defer os.Remove(tempPath)

		case proto.CopyMsg_DATA:
			if tempFile != nil {
				tempFile.Write(msg.GetData())
			}
		}
	}
}

func (s *netemServer) Close() error {
	var ids []string
	for _, project := range GetAllProjects() {
		ids = append(ids, project.Id)
	}

	for _, prjId := range ids {
		if err := CloseProject(prjId); err != nil {
			logrus.Errorf("Error when closing project %s: %v", prjId, err)
		}
	}

	return nil
}

func NewServer() *netemServer {
	return &netemServer{}
}
