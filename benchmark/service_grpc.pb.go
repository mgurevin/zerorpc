// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package benchmark

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion7

// SumClient is the client API for Sum service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type SumClient interface {
	Sum(ctx context.Context, in *SumRequest, opts ...grpc.CallOption) (*SumResponse, error)
}

type sumClient struct {
	cc grpc.ClientConnInterface
}

func NewSumClient(cc grpc.ClientConnInterface) SumClient {
	return &sumClient{cc}
}

func (c *sumClient) Sum(ctx context.Context, in *SumRequest, opts ...grpc.CallOption) (*SumResponse, error) {
	out := new(SumResponse)
	err := c.cc.Invoke(ctx, "/Sum/Sum", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SumServer is the server API for Sum service.
// All implementations must embed UnimplementedSumServer
// for forward compatibility
type SumServer interface {
	Sum(context.Context, *SumRequest) (*SumResponse, error)
	mustEmbedUnimplementedSumServer()
}

// UnimplementedSumServer must be embedded to have forward compatible implementations.
type UnimplementedSumServer struct {
}

func (UnimplementedSumServer) Sum(context.Context, *SumRequest) (*SumResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Sum not implemented")
}
func (UnimplementedSumServer) mustEmbedUnimplementedSumServer() {}

// UnsafeSumServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to SumServer will
// result in compilation errors.
type UnsafeSumServer interface {
	mustEmbedUnimplementedSumServer()
}

func RegisterSumServer(s grpc.ServiceRegistrar, srv SumServer) {
	s.RegisterService(&_Sum_serviceDesc, srv)
}

func _Sum_Sum_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SumRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SumServer).Sum(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/Sum/Sum",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SumServer).Sum(ctx, req.(*SumRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _Sum_serviceDesc = grpc.ServiceDesc{
	ServiceName: "Sum",
	HandlerType: (*SumServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Sum",
			Handler:    _Sum_Sum_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "service.proto",
}
