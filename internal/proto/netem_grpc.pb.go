// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v4.25.3
// source: internal/proto/netem.proto

package proto

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	Netem_GetVersion_FullMethodName        = "/netem.Netem/GetVersion"
	Netem_PullImages_FullMethodName        = "/netem.Netem/PullImages"
	Netem_Clean_FullMethodName             = "/netem.Netem/Clean"
	Netem_GetProjects_FullMethodName       = "/netem.Netem/GetProjects"
	Netem_OpenProject_FullMethodName       = "/netem.Netem/OpenProject"
	Netem_CloseProject_FullMethodName      = "/netem.Netem/CloseProject"
	Netem_SaveProject_FullMethodName       = "/netem.Netem/SaveProject"
	Netem_GetProjectConfigs_FullMethodName = "/netem.Netem/GetProjectConfigs"
	Netem_GetProjectStatus_FullMethodName  = "/netem.Netem/GetProjectStatus"
	Netem_ReadNetworkFile_FullMethodName   = "/netem.Netem/ReadNetworkFile"
	Netem_WriteNetworkFile_FullMethodName  = "/netem.Netem/WriteNetworkFile"
	Netem_Check_FullMethodName             = "/netem.Netem/Check"
	Netem_Reload_FullMethodName            = "/netem.Netem/Reload"
	Netem_Run_FullMethodName               = "/netem.Netem/Run"
	Netem_ReadConfigFiles_FullMethodName   = "/netem.Netem/ReadConfigFiles"
	Netem_CanRunConsole_FullMethodName     = "/netem.Netem/CanRunConsole"
	Netem_Console_FullMethodName           = "/netem.Netem/Console"
	Netem_Start_FullMethodName             = "/netem.Netem/Start"
	Netem_Stop_FullMethodName              = "/netem.Netem/Stop"
	Netem_Restart_FullMethodName           = "/netem.Netem/Restart"
	Netem_SetIfState_FullMethodName        = "/netem.Netem/SetIfState"
	Netem_Capture_FullMethodName           = "/netem.Netem/Capture"
	Netem_CopyFrom_FullMethodName          = "/netem.Netem/CopyFrom"
	Netem_CopyTo_FullMethodName            = "/netem.Netem/CopyTo"
	Netem_LinkUpdate_FullMethodName        = "/netem.Netem/LinkUpdate"
)

// NetemClient is the client API for Netem service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type NetemClient interface {
	// general action
	GetVersion(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*VersionResponse, error)
	PullImages(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (Netem_PullImagesClient, error)
	Clean(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*AckResponse, error)
	// Project actions
	GetProjects(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*PrjListResponse, error)
	OpenProject(ctx context.Context, in *OpenRequest, opts ...grpc.CallOption) (*PrjOpenResponse, error)
	CloseProject(ctx context.Context, in *ProjectRequest, opts ...grpc.CallOption) (*AckResponse, error)
	SaveProject(ctx context.Context, in *ProjectRequest, opts ...grpc.CallOption) (*FileResponse, error)
	GetProjectConfigs(ctx context.Context, in *ProjectRequest, opts ...grpc.CallOption) (*FileResponse, error)
	GetProjectStatus(ctx context.Context, in *ProjectRequest, opts ...grpc.CallOption) (*StatusResponse, error)
	// Read/Write network topology
	ReadNetworkFile(ctx context.Context, in *ProjectRequest, opts ...grpc.CallOption) (*FileResponse, error)
	WriteNetworkFile(ctx context.Context, in *WNetworkRequest, opts ...grpc.CallOption) (*AckResponse, error)
	// topology actions
	Check(ctx context.Context, in *ProjectRequest, opts ...grpc.CallOption) (*AckResponse, error)
	Reload(ctx context.Context, in *ProjectRequest, opts ...grpc.CallOption) (*RunResponse, error)
	Run(ctx context.Context, in *ProjectRequest, opts ...grpc.CallOption) (*RunResponse, error)
	// Node actions
	ReadConfigFiles(ctx context.Context, in *NodeRequest, opts ...grpc.CallOption) (*ConfigFilesResponse, error)
	CanRunConsole(ctx context.Context, in *NodeRequest, opts ...grpc.CallOption) (*AckResponse, error)
	Console(ctx context.Context, opts ...grpc.CallOption) (Netem_ConsoleClient, error)
	Start(ctx context.Context, in *NodeRequest, opts ...grpc.CallOption) (*AckResponse, error)
	Stop(ctx context.Context, in *NodeRequest, opts ...grpc.CallOption) (*AckResponse, error)
	Restart(ctx context.Context, in *NodeRequest, opts ...grpc.CallOption) (*AckResponse, error)
	SetIfState(ctx context.Context, in *NodeIfStateRequest, opts ...grpc.CallOption) (*AckResponse, error)
	Capture(ctx context.Context, in *NodeInterfaceRequest, opts ...grpc.CallOption) (Netem_CaptureClient, error)
	CopyFrom(ctx context.Context, in *CopyMsg, opts ...grpc.CallOption) (Netem_CopyFromClient, error)
	CopyTo(ctx context.Context, opts ...grpc.CallOption) (Netem_CopyToClient, error)
	// Link actions
	LinkUpdate(ctx context.Context, in *LinkRequest, opts ...grpc.CallOption) (*AckResponse, error)
}

type netemClient struct {
	cc grpc.ClientConnInterface
}

func NewNetemClient(cc grpc.ClientConnInterface) NetemClient {
	return &netemClient{cc}
}

func (c *netemClient) GetVersion(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*VersionResponse, error) {
	out := new(VersionResponse)
	err := c.cc.Invoke(ctx, Netem_GetVersion_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *netemClient) PullImages(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (Netem_PullImagesClient, error) {
	stream, err := c.cc.NewStream(ctx, &Netem_ServiceDesc.Streams[0], Netem_PullImages_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &netemPullImagesClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Netem_PullImagesClient interface {
	Recv() (*PullSrvMsg, error)
	grpc.ClientStream
}

type netemPullImagesClient struct {
	grpc.ClientStream
}

func (x *netemPullImagesClient) Recv() (*PullSrvMsg, error) {
	m := new(PullSrvMsg)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *netemClient) Clean(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*AckResponse, error) {
	out := new(AckResponse)
	err := c.cc.Invoke(ctx, Netem_Clean_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *netemClient) GetProjects(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*PrjListResponse, error) {
	out := new(PrjListResponse)
	err := c.cc.Invoke(ctx, Netem_GetProjects_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *netemClient) OpenProject(ctx context.Context, in *OpenRequest, opts ...grpc.CallOption) (*PrjOpenResponse, error) {
	out := new(PrjOpenResponse)
	err := c.cc.Invoke(ctx, Netem_OpenProject_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *netemClient) CloseProject(ctx context.Context, in *ProjectRequest, opts ...grpc.CallOption) (*AckResponse, error) {
	out := new(AckResponse)
	err := c.cc.Invoke(ctx, Netem_CloseProject_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *netemClient) SaveProject(ctx context.Context, in *ProjectRequest, opts ...grpc.CallOption) (*FileResponse, error) {
	out := new(FileResponse)
	err := c.cc.Invoke(ctx, Netem_SaveProject_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *netemClient) GetProjectConfigs(ctx context.Context, in *ProjectRequest, opts ...grpc.CallOption) (*FileResponse, error) {
	out := new(FileResponse)
	err := c.cc.Invoke(ctx, Netem_GetProjectConfigs_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *netemClient) GetProjectStatus(ctx context.Context, in *ProjectRequest, opts ...grpc.CallOption) (*StatusResponse, error) {
	out := new(StatusResponse)
	err := c.cc.Invoke(ctx, Netem_GetProjectStatus_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *netemClient) ReadNetworkFile(ctx context.Context, in *ProjectRequest, opts ...grpc.CallOption) (*FileResponse, error) {
	out := new(FileResponse)
	err := c.cc.Invoke(ctx, Netem_ReadNetworkFile_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *netemClient) WriteNetworkFile(ctx context.Context, in *WNetworkRequest, opts ...grpc.CallOption) (*AckResponse, error) {
	out := new(AckResponse)
	err := c.cc.Invoke(ctx, Netem_WriteNetworkFile_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *netemClient) Check(ctx context.Context, in *ProjectRequest, opts ...grpc.CallOption) (*AckResponse, error) {
	out := new(AckResponse)
	err := c.cc.Invoke(ctx, Netem_Check_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *netemClient) Reload(ctx context.Context, in *ProjectRequest, opts ...grpc.CallOption) (*RunResponse, error) {
	out := new(RunResponse)
	err := c.cc.Invoke(ctx, Netem_Reload_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *netemClient) Run(ctx context.Context, in *ProjectRequest, opts ...grpc.CallOption) (*RunResponse, error) {
	out := new(RunResponse)
	err := c.cc.Invoke(ctx, Netem_Run_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *netemClient) ReadConfigFiles(ctx context.Context, in *NodeRequest, opts ...grpc.CallOption) (*ConfigFilesResponse, error) {
	out := new(ConfigFilesResponse)
	err := c.cc.Invoke(ctx, Netem_ReadConfigFiles_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *netemClient) CanRunConsole(ctx context.Context, in *NodeRequest, opts ...grpc.CallOption) (*AckResponse, error) {
	out := new(AckResponse)
	err := c.cc.Invoke(ctx, Netem_CanRunConsole_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *netemClient) Console(ctx context.Context, opts ...grpc.CallOption) (Netem_ConsoleClient, error) {
	stream, err := c.cc.NewStream(ctx, &Netem_ServiceDesc.Streams[1], Netem_Console_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &netemConsoleClient{stream}
	return x, nil
}

type Netem_ConsoleClient interface {
	Send(*ConsoleCltMsg) error
	Recv() (*ConsoleSrvMsg, error)
	grpc.ClientStream
}

type netemConsoleClient struct {
	grpc.ClientStream
}

func (x *netemConsoleClient) Send(m *ConsoleCltMsg) error {
	return x.ClientStream.SendMsg(m)
}

func (x *netemConsoleClient) Recv() (*ConsoleSrvMsg, error) {
	m := new(ConsoleSrvMsg)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *netemClient) Start(ctx context.Context, in *NodeRequest, opts ...grpc.CallOption) (*AckResponse, error) {
	out := new(AckResponse)
	err := c.cc.Invoke(ctx, Netem_Start_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *netemClient) Stop(ctx context.Context, in *NodeRequest, opts ...grpc.CallOption) (*AckResponse, error) {
	out := new(AckResponse)
	err := c.cc.Invoke(ctx, Netem_Stop_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *netemClient) Restart(ctx context.Context, in *NodeRequest, opts ...grpc.CallOption) (*AckResponse, error) {
	out := new(AckResponse)
	err := c.cc.Invoke(ctx, Netem_Restart_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *netemClient) SetIfState(ctx context.Context, in *NodeIfStateRequest, opts ...grpc.CallOption) (*AckResponse, error) {
	out := new(AckResponse)
	err := c.cc.Invoke(ctx, Netem_SetIfState_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *netemClient) Capture(ctx context.Context, in *NodeInterfaceRequest, opts ...grpc.CallOption) (Netem_CaptureClient, error) {
	stream, err := c.cc.NewStream(ctx, &Netem_ServiceDesc.Streams[2], Netem_Capture_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &netemCaptureClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Netem_CaptureClient interface {
	Recv() (*CaptureSrvMsg, error)
	grpc.ClientStream
}

type netemCaptureClient struct {
	grpc.ClientStream
}

func (x *netemCaptureClient) Recv() (*CaptureSrvMsg, error) {
	m := new(CaptureSrvMsg)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *netemClient) CopyFrom(ctx context.Context, in *CopyMsg, opts ...grpc.CallOption) (Netem_CopyFromClient, error) {
	stream, err := c.cc.NewStream(ctx, &Netem_ServiceDesc.Streams[3], Netem_CopyFrom_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &netemCopyFromClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Netem_CopyFromClient interface {
	Recv() (*CopyMsg, error)
	grpc.ClientStream
}

type netemCopyFromClient struct {
	grpc.ClientStream
}

func (x *netemCopyFromClient) Recv() (*CopyMsg, error) {
	m := new(CopyMsg)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *netemClient) CopyTo(ctx context.Context, opts ...grpc.CallOption) (Netem_CopyToClient, error) {
	stream, err := c.cc.NewStream(ctx, &Netem_ServiceDesc.Streams[4], Netem_CopyTo_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &netemCopyToClient{stream}
	return x, nil
}

type Netem_CopyToClient interface {
	Send(*CopyMsg) error
	CloseAndRecv() (*AckResponse, error)
	grpc.ClientStream
}

type netemCopyToClient struct {
	grpc.ClientStream
}

func (x *netemCopyToClient) Send(m *CopyMsg) error {
	return x.ClientStream.SendMsg(m)
}

func (x *netemCopyToClient) CloseAndRecv() (*AckResponse, error) {
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	m := new(AckResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *netemClient) LinkUpdate(ctx context.Context, in *LinkRequest, opts ...grpc.CallOption) (*AckResponse, error) {
	out := new(AckResponse)
	err := c.cc.Invoke(ctx, Netem_LinkUpdate_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// NetemServer is the server API for Netem service.
// All implementations must embed UnimplementedNetemServer
// for forward compatibility
type NetemServer interface {
	// general action
	GetVersion(context.Context, *emptypb.Empty) (*VersionResponse, error)
	PullImages(*emptypb.Empty, Netem_PullImagesServer) error
	Clean(context.Context, *emptypb.Empty) (*AckResponse, error)
	// Project actions
	GetProjects(context.Context, *emptypb.Empty) (*PrjListResponse, error)
	OpenProject(context.Context, *OpenRequest) (*PrjOpenResponse, error)
	CloseProject(context.Context, *ProjectRequest) (*AckResponse, error)
	SaveProject(context.Context, *ProjectRequest) (*FileResponse, error)
	GetProjectConfigs(context.Context, *ProjectRequest) (*FileResponse, error)
	GetProjectStatus(context.Context, *ProjectRequest) (*StatusResponse, error)
	// Read/Write network topology
	ReadNetworkFile(context.Context, *ProjectRequest) (*FileResponse, error)
	WriteNetworkFile(context.Context, *WNetworkRequest) (*AckResponse, error)
	// topology actions
	Check(context.Context, *ProjectRequest) (*AckResponse, error)
	Reload(context.Context, *ProjectRequest) (*RunResponse, error)
	Run(context.Context, *ProjectRequest) (*RunResponse, error)
	// Node actions
	ReadConfigFiles(context.Context, *NodeRequest) (*ConfigFilesResponse, error)
	CanRunConsole(context.Context, *NodeRequest) (*AckResponse, error)
	Console(Netem_ConsoleServer) error
	Start(context.Context, *NodeRequest) (*AckResponse, error)
	Stop(context.Context, *NodeRequest) (*AckResponse, error)
	Restart(context.Context, *NodeRequest) (*AckResponse, error)
	SetIfState(context.Context, *NodeIfStateRequest) (*AckResponse, error)
	Capture(*NodeInterfaceRequest, Netem_CaptureServer) error
	CopyFrom(*CopyMsg, Netem_CopyFromServer) error
	CopyTo(Netem_CopyToServer) error
	// Link actions
	LinkUpdate(context.Context, *LinkRequest) (*AckResponse, error)
	mustEmbedUnimplementedNetemServer()
}

// UnimplementedNetemServer must be embedded to have forward compatible implementations.
type UnimplementedNetemServer struct {
}

func (UnimplementedNetemServer) GetVersion(context.Context, *emptypb.Empty) (*VersionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetVersion not implemented")
}
func (UnimplementedNetemServer) PullImages(*emptypb.Empty, Netem_PullImagesServer) error {
	return status.Errorf(codes.Unimplemented, "method PullImages not implemented")
}
func (UnimplementedNetemServer) Clean(context.Context, *emptypb.Empty) (*AckResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Clean not implemented")
}
func (UnimplementedNetemServer) GetProjects(context.Context, *emptypb.Empty) (*PrjListResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetProjects not implemented")
}
func (UnimplementedNetemServer) OpenProject(context.Context, *OpenRequest) (*PrjOpenResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method OpenProject not implemented")
}
func (UnimplementedNetemServer) CloseProject(context.Context, *ProjectRequest) (*AckResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CloseProject not implemented")
}
func (UnimplementedNetemServer) SaveProject(context.Context, *ProjectRequest) (*FileResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SaveProject not implemented")
}
func (UnimplementedNetemServer) GetProjectConfigs(context.Context, *ProjectRequest) (*FileResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetProjectConfigs not implemented")
}
func (UnimplementedNetemServer) GetProjectStatus(context.Context, *ProjectRequest) (*StatusResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetProjectStatus not implemented")
}
func (UnimplementedNetemServer) ReadNetworkFile(context.Context, *ProjectRequest) (*FileResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReadNetworkFile not implemented")
}
func (UnimplementedNetemServer) WriteNetworkFile(context.Context, *WNetworkRequest) (*AckResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method WriteNetworkFile not implemented")
}
func (UnimplementedNetemServer) Check(context.Context, *ProjectRequest) (*AckResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Check not implemented")
}
func (UnimplementedNetemServer) Reload(context.Context, *ProjectRequest) (*RunResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Reload not implemented")
}
func (UnimplementedNetemServer) Run(context.Context, *ProjectRequest) (*RunResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Run not implemented")
}
func (UnimplementedNetemServer) ReadConfigFiles(context.Context, *NodeRequest) (*ConfigFilesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReadConfigFiles not implemented")
}
func (UnimplementedNetemServer) CanRunConsole(context.Context, *NodeRequest) (*AckResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CanRunConsole not implemented")
}
func (UnimplementedNetemServer) Console(Netem_ConsoleServer) error {
	return status.Errorf(codes.Unimplemented, "method Console not implemented")
}
func (UnimplementedNetemServer) Start(context.Context, *NodeRequest) (*AckResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Start not implemented")
}
func (UnimplementedNetemServer) Stop(context.Context, *NodeRequest) (*AckResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Stop not implemented")
}
func (UnimplementedNetemServer) Restart(context.Context, *NodeRequest) (*AckResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Restart not implemented")
}
func (UnimplementedNetemServer) SetIfState(context.Context, *NodeIfStateRequest) (*AckResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetIfState not implemented")
}
func (UnimplementedNetemServer) Capture(*NodeInterfaceRequest, Netem_CaptureServer) error {
	return status.Errorf(codes.Unimplemented, "method Capture not implemented")
}
func (UnimplementedNetemServer) CopyFrom(*CopyMsg, Netem_CopyFromServer) error {
	return status.Errorf(codes.Unimplemented, "method CopyFrom not implemented")
}
func (UnimplementedNetemServer) CopyTo(Netem_CopyToServer) error {
	return status.Errorf(codes.Unimplemented, "method CopyTo not implemented")
}
func (UnimplementedNetemServer) LinkUpdate(context.Context, *LinkRequest) (*AckResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method LinkUpdate not implemented")
}
func (UnimplementedNetemServer) mustEmbedUnimplementedNetemServer() {}

// UnsafeNetemServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to NetemServer will
// result in compilation errors.
type UnsafeNetemServer interface {
	mustEmbedUnimplementedNetemServer()
}

func RegisterNetemServer(s grpc.ServiceRegistrar, srv NetemServer) {
	s.RegisterService(&Netem_ServiceDesc, srv)
}

func _Netem_GetVersion_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetemServer).GetVersion(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Netem_GetVersion_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetemServer).GetVersion(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _Netem_PullImages_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(emptypb.Empty)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(NetemServer).PullImages(m, &netemPullImagesServer{stream})
}

type Netem_PullImagesServer interface {
	Send(*PullSrvMsg) error
	grpc.ServerStream
}

type netemPullImagesServer struct {
	grpc.ServerStream
}

func (x *netemPullImagesServer) Send(m *PullSrvMsg) error {
	return x.ServerStream.SendMsg(m)
}

func _Netem_Clean_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetemServer).Clean(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Netem_Clean_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetemServer).Clean(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _Netem_GetProjects_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetemServer).GetProjects(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Netem_GetProjects_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetemServer).GetProjects(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _Netem_OpenProject_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(OpenRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetemServer).OpenProject(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Netem_OpenProject_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetemServer).OpenProject(ctx, req.(*OpenRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Netem_CloseProject_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ProjectRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetemServer).CloseProject(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Netem_CloseProject_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetemServer).CloseProject(ctx, req.(*ProjectRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Netem_SaveProject_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ProjectRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetemServer).SaveProject(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Netem_SaveProject_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetemServer).SaveProject(ctx, req.(*ProjectRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Netem_GetProjectConfigs_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ProjectRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetemServer).GetProjectConfigs(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Netem_GetProjectConfigs_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetemServer).GetProjectConfigs(ctx, req.(*ProjectRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Netem_GetProjectStatus_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ProjectRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetemServer).GetProjectStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Netem_GetProjectStatus_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetemServer).GetProjectStatus(ctx, req.(*ProjectRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Netem_ReadNetworkFile_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ProjectRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetemServer).ReadNetworkFile(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Netem_ReadNetworkFile_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetemServer).ReadNetworkFile(ctx, req.(*ProjectRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Netem_WriteNetworkFile_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(WNetworkRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetemServer).WriteNetworkFile(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Netem_WriteNetworkFile_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetemServer).WriteNetworkFile(ctx, req.(*WNetworkRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Netem_Check_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ProjectRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetemServer).Check(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Netem_Check_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetemServer).Check(ctx, req.(*ProjectRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Netem_Reload_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ProjectRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetemServer).Reload(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Netem_Reload_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetemServer).Reload(ctx, req.(*ProjectRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Netem_Run_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ProjectRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetemServer).Run(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Netem_Run_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetemServer).Run(ctx, req.(*ProjectRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Netem_ReadConfigFiles_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(NodeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetemServer).ReadConfigFiles(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Netem_ReadConfigFiles_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetemServer).ReadConfigFiles(ctx, req.(*NodeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Netem_CanRunConsole_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(NodeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetemServer).CanRunConsole(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Netem_CanRunConsole_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetemServer).CanRunConsole(ctx, req.(*NodeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Netem_Console_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(NetemServer).Console(&netemConsoleServer{stream})
}

type Netem_ConsoleServer interface {
	Send(*ConsoleSrvMsg) error
	Recv() (*ConsoleCltMsg, error)
	grpc.ServerStream
}

type netemConsoleServer struct {
	grpc.ServerStream
}

func (x *netemConsoleServer) Send(m *ConsoleSrvMsg) error {
	return x.ServerStream.SendMsg(m)
}

func (x *netemConsoleServer) Recv() (*ConsoleCltMsg, error) {
	m := new(ConsoleCltMsg)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _Netem_Start_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(NodeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetemServer).Start(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Netem_Start_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetemServer).Start(ctx, req.(*NodeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Netem_Stop_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(NodeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetemServer).Stop(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Netem_Stop_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetemServer).Stop(ctx, req.(*NodeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Netem_Restart_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(NodeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetemServer).Restart(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Netem_Restart_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetemServer).Restart(ctx, req.(*NodeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Netem_SetIfState_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(NodeIfStateRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetemServer).SetIfState(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Netem_SetIfState_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetemServer).SetIfState(ctx, req.(*NodeIfStateRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Netem_Capture_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(NodeInterfaceRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(NetemServer).Capture(m, &netemCaptureServer{stream})
}

type Netem_CaptureServer interface {
	Send(*CaptureSrvMsg) error
	grpc.ServerStream
}

type netemCaptureServer struct {
	grpc.ServerStream
}

func (x *netemCaptureServer) Send(m *CaptureSrvMsg) error {
	return x.ServerStream.SendMsg(m)
}

func _Netem_CopyFrom_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(CopyMsg)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(NetemServer).CopyFrom(m, &netemCopyFromServer{stream})
}

type Netem_CopyFromServer interface {
	Send(*CopyMsg) error
	grpc.ServerStream
}

type netemCopyFromServer struct {
	grpc.ServerStream
}

func (x *netemCopyFromServer) Send(m *CopyMsg) error {
	return x.ServerStream.SendMsg(m)
}

func _Netem_CopyTo_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(NetemServer).CopyTo(&netemCopyToServer{stream})
}

type Netem_CopyToServer interface {
	SendAndClose(*AckResponse) error
	Recv() (*CopyMsg, error)
	grpc.ServerStream
}

type netemCopyToServer struct {
	grpc.ServerStream
}

func (x *netemCopyToServer) SendAndClose(m *AckResponse) error {
	return x.ServerStream.SendMsg(m)
}

func (x *netemCopyToServer) Recv() (*CopyMsg, error) {
	m := new(CopyMsg)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _Netem_LinkUpdate_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LinkRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetemServer).LinkUpdate(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Netem_LinkUpdate_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetemServer).LinkUpdate(ctx, req.(*LinkRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Netem_ServiceDesc is the grpc.ServiceDesc for Netem service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Netem_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "netem.Netem",
	HandlerType: (*NetemServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetVersion",
			Handler:    _Netem_GetVersion_Handler,
		},
		{
			MethodName: "Clean",
			Handler:    _Netem_Clean_Handler,
		},
		{
			MethodName: "GetProjects",
			Handler:    _Netem_GetProjects_Handler,
		},
		{
			MethodName: "OpenProject",
			Handler:    _Netem_OpenProject_Handler,
		},
		{
			MethodName: "CloseProject",
			Handler:    _Netem_CloseProject_Handler,
		},
		{
			MethodName: "SaveProject",
			Handler:    _Netem_SaveProject_Handler,
		},
		{
			MethodName: "GetProjectConfigs",
			Handler:    _Netem_GetProjectConfigs_Handler,
		},
		{
			MethodName: "GetProjectStatus",
			Handler:    _Netem_GetProjectStatus_Handler,
		},
		{
			MethodName: "ReadNetworkFile",
			Handler:    _Netem_ReadNetworkFile_Handler,
		},
		{
			MethodName: "WriteNetworkFile",
			Handler:    _Netem_WriteNetworkFile_Handler,
		},
		{
			MethodName: "Check",
			Handler:    _Netem_Check_Handler,
		},
		{
			MethodName: "Reload",
			Handler:    _Netem_Reload_Handler,
		},
		{
			MethodName: "Run",
			Handler:    _Netem_Run_Handler,
		},
		{
			MethodName: "ReadConfigFiles",
			Handler:    _Netem_ReadConfigFiles_Handler,
		},
		{
			MethodName: "CanRunConsole",
			Handler:    _Netem_CanRunConsole_Handler,
		},
		{
			MethodName: "Start",
			Handler:    _Netem_Start_Handler,
		},
		{
			MethodName: "Stop",
			Handler:    _Netem_Stop_Handler,
		},
		{
			MethodName: "Restart",
			Handler:    _Netem_Restart_Handler,
		},
		{
			MethodName: "SetIfState",
			Handler:    _Netem_SetIfState_Handler,
		},
		{
			MethodName: "LinkUpdate",
			Handler:    _Netem_LinkUpdate_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "PullImages",
			Handler:       _Netem_PullImages_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "Console",
			Handler:       _Netem_Console_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
		{
			StreamName:    "Capture",
			Handler:       _Netem_Capture_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "CopyFrom",
			Handler:       _Netem_CopyFrom_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "CopyTo",
			Handler:       _Netem_CopyTo_Handler,
			ClientStreams: true,
		},
	},
	Metadata: "internal/proto/netem.proto",
}
