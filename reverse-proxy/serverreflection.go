package reverse_proxy

import (
	"context"
	"google.golang.org/grpc"
	grpcReflection "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"io"
	"strings"
	"time"
)

func (grp *GrpcReverseProxy) getEndpointServerReflectionResponse(reqCtx context.Context, req *grpcReflection.ServerReflectionRequest, endpoint string) (*grpcReflection.ServerReflectionResponse, error) {
	var conn *grpc.ClientConn
	var err error
	dialCtx, cls := context.WithTimeout(context.Background(), time.Second*3)
	defer cls()
	conn, err = grp.DialBackend(dialCtx, endpoint)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	c := grpcReflection.NewServerReflectionClient(conn)
	var infoC grpcReflection.ServerReflection_ServerReflectionInfoClient
	infoC, err = c.ServerReflectionInfo(reqCtx)
	if err != nil {
		return nil, err
	}
	err = infoC.Send(req)
	if err != nil {
		return nil, err
	}
	return infoC.Recv()
}
func (grp *GrpcReverseProxy) ServerReflectionInfo(stream grpcReflection.ServerReflection_ServerReflectionInfoServer) error {
	sis, err := grp.opts.BackendDiscovery.ListServices()
	if err != nil {
		return err
	}
	var uniqueGrpcServiceEndpoints []string
	for _, si := range sis {
	SearchEP:
		for _, sie := range si {
			for _, ep := range sie.Endpoints {
				if strings.Index(ep, "grpc://") == 0 {
					uniqueGrpcServiceEndpoints = append(uniqueGrpcServiceEndpoints, ep[7:])
					break SearchEP
				}
			}
		}
	}

	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		out := &grpcReflection.ServerReflectionResponse{
			ValidHost:       in.Host,
			OriginalRequest: in,
		}
		nReq := &grpcReflection.ServerReflectionRequest{}
		var resp *grpcReflection.ServerReflectionResponse
		switch req := in.MessageRequest.(type) {
		case *grpcReflection.ServerReflectionRequest_FileByFilename:
			reqM := &grpcReflection.ServerReflectionRequest_FileByFilename{
				FileByFilename: req.FileByFilename,
			}
			var respFileDescriptorProto [][]byte
			nReq.Host = in.Host
			nReq.MessageRequest = reqM
			for _, ep := range uniqueGrpcServiceEndpoints {
				resp, err = grp.getEndpointServerReflectionResponse(stream.Context(), nReq, ep)
				if err != nil {
					return err
				}
				if resp.GetFileDescriptorResponse() != nil {
					respFileDescriptorProto = append(respFileDescriptorProto, resp.GetFileDescriptorResponse().FileDescriptorProto...)
				}
			}
			out.MessageResponse = &grpcReflection.ServerReflectionResponse_FileDescriptorResponse{
				FileDescriptorResponse: &grpcReflection.FileDescriptorResponse{FileDescriptorProto: respFileDescriptorProto},
			}

		case *grpcReflection.ServerReflectionRequest_FileContainingSymbol:
			reqM := &grpcReflection.ServerReflectionRequest_FileContainingSymbol{
				FileContainingSymbol: req.FileContainingSymbol,
			}
			var respFileDescriptorProto [][]byte
			nReq.Host = in.Host
			nReq.MessageRequest = reqM
			for _, ep := range uniqueGrpcServiceEndpoints {
				resp, err = grp.getEndpointServerReflectionResponse(stream.Context(), nReq, ep)
				if err != nil {
					return err
				}
				if resp.GetFileDescriptorResponse() != nil {
					respFileDescriptorProto = append(respFileDescriptorProto, resp.GetFileDescriptorResponse().FileDescriptorProto...)
				}
			}
			out.MessageResponse = &grpcReflection.ServerReflectionResponse_FileDescriptorResponse{
				FileDescriptorResponse: &grpcReflection.FileDescriptorResponse{FileDescriptorProto: respFileDescriptorProto},
			}
		case *grpcReflection.ServerReflectionRequest_FileContainingExtension:
			reqM := &grpcReflection.ServerReflectionRequest_FileContainingExtension{
				FileContainingExtension: req.FileContainingExtension,
			}
			var respFileDescriptorProto [][]byte
			nReq.Host = in.Host
			nReq.MessageRequest = reqM
			for _, ep := range uniqueGrpcServiceEndpoints {
				resp, err = grp.getEndpointServerReflectionResponse(stream.Context(), nReq, ep)
				if err != nil {
					return err
				}
				if resp.GetFileDescriptorResponse() != nil {
					respFileDescriptorProto = append(respFileDescriptorProto, resp.GetFileDescriptorResponse().FileDescriptorProto...)
				}
			}
			out.MessageResponse = &grpcReflection.ServerReflectionResponse_FileDescriptorResponse{
				FileDescriptorResponse: &grpcReflection.FileDescriptorResponse{FileDescriptorProto: respFileDescriptorProto},
			}
		case *grpcReflection.ServerReflectionRequest_AllExtensionNumbersOfType:
			reqM := &grpcReflection.ServerReflectionRequest_AllExtensionNumbersOfType{
				AllExtensionNumbersOfType: req.AllExtensionNumbersOfType,
			}
			var extNumbers []int32
			nReq.Host = in.Host
			nReq.MessageRequest = reqM
			for _, ep := range uniqueGrpcServiceEndpoints {
				resp, err = grp.getEndpointServerReflectionResponse(stream.Context(), nReq, ep)
				if err != nil {
					return err
				}
				extNumbers = append(extNumbers, resp.GetAllExtensionNumbersResponse().ExtensionNumber...)
			}
			out.MessageResponse = &grpcReflection.ServerReflectionResponse_AllExtensionNumbersResponse{
				AllExtensionNumbersResponse: &grpcReflection.ExtensionNumberResponse{
					ExtensionNumber: extNumbers,
					BaseTypeName:    resp.GetAllExtensionNumbersResponse().GetBaseTypeName(),
				},
			}
		case *grpcReflection.ServerReflectionRequest_ListServices:
			reqM := &grpcReflection.ServerReflectionRequest_ListServices{
				ListServices: req.ListServices,
			}
			var svcList []*grpcReflection.ServiceResponse
			nReq.Host = in.Host
			nReq.MessageRequest = reqM
			for _, ep := range uniqueGrpcServiceEndpoints {
				resp, err = grp.getEndpointServerReflectionResponse(stream.Context(), nReq, ep)
				if err != nil {
					return err
				}
				svcList = append(svcList, resp.GetListServicesResponse().Service...)
			}
			out.MessageResponse = &grpcReflection.ServerReflectionResponse_ListServicesResponse{
				ListServicesResponse: &grpcReflection.ListServiceResponse{
					Service: svcList,
				},
			}
		}
		if err := stream.Send(out); err != nil {
			return err
		}
	}
}
