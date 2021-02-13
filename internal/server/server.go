package server

import (
	context "context"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/mroy31/gonetem/internal/logger"
	"github.com/mroy31/gonetem/internal/options"
	"github.com/mroy31/gonetem/internal/proto"
	"github.com/mroy31/gonetem/internal/utils"
)

type netemServer struct {
	proto.UnimplementedNetemServer
}

func (s *netemServer) GetVersion(ctx context.Context, empty *empty.Empty) (*proto.VersionResponse, error) {
	logger.Debug("msg", "Receive GetVersion RPC")
	return &proto.VersionResponse{
		Status: &proto.Status{
			Code: proto.StatusCode_OK,
		},
		Version: options.VERSION,
	}, nil
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
			OpenAt: prj.OpenAt.String(),
		})
	}

	return &response, nil
}

func (s *netemServer) OpenProject(ctx context.Context, request *proto.OpenRequest) (*proto.PrjOpenResponse, error) {
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

func (s *netemServer) GetProjectStatus(ctx context.Context, request *proto.ProjectRequest) (*proto.StatusResponse, error) {
	project := GetProject(request.GetId())
	if project == nil {
		return nil, &ProjectNotFoundError{request.GetId()}
	}

	response := &proto.StatusResponse{
		Status:  &proto.Status{Code: proto.StatusCode_OK},
		Name:    project.Name,
		Id:      project.Id,
		OpenAt:  project.OpenAt.String(),
		Running: project.Topology.IsRunning(),
	}

	if project.Topology.IsRunning() {
		for _, node := range project.Topology.GetAllNodes() {
			response.Nodes = append(response.Nodes, &proto.StatusResponse_NodeStatus{
				Name:    node.GetName(),
				Running: node.IsRunning(),
			})
		}
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

func (s *netemServer) Run(ctx context.Context, request *proto.ProjectRequest) (*proto.AckResponse, error) {
	project := GetProject(request.GetId())
	if project == nil {
		return nil, &ProjectNotFoundError{request.GetId()}
	}

	if err := project.Topology.Run(); err != nil {
		return nil, err
	}

	return &proto.AckResponse{
		Status: &proto.Status{Code: proto.StatusCode_OK},
	}, nil
}

func (s *netemServer) Reload(ctx context.Context, request *proto.ProjectRequest) (*proto.AckResponse, error) {
	project := GetProject(request.GetId())
	if project == nil {
		return nil, &ProjectNotFoundError{request.GetId()}
	}

	if err := project.Topology.Reload(); err != nil {
		return nil, err
	}

	return &proto.AckResponse{
		Status: &proto.Status{Code: proto.StatusCode_OK},
	}, nil
}

func (s *netemServer) Console(stream proto.Netem_ConsoleServer) error {
	return nil
}

func NewServer() *netemServer {
	return &netemServer{}
}
