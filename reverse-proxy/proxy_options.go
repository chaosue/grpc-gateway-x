package reverse_proxy

import (
	"grpc-gateway-x/discovery"
)

// WithEndpointParser set the parser to parse server endpoint from the grpc request path. if WithBackendAddr option is set, the parser won't be used.
func WithEndpointParser(parser EndpointParser) GrpcReverseProxyOption {
	return func(opts *GrpcReverseProxyOptions) {
		opts.EndpointParser = parser
	}
}

func WithBackendInsecure(sec bool) GrpcReverseProxyOption {
	return func(opts *GrpcReverseProxyOptions) {
		opts.BackendInsecure = sec
	}
}

// WithBackendAddr set the option BackendAddr, e.g.
//
//	"127.0.0.1:3212"
func WithBackendAddr(addr string) GrpcReverseProxyOption {
	return func(opts *GrpcReverseProxyOptions) {
		opts.BackendAddr = addr
	}
}

func WithBackendDiscovery(d discovery.Discovery) GrpcReverseProxyOption {
	return func(opts *GrpcReverseProxyOptions) {
		opts.BackendDiscovery = d
	}
}

func WithBackendTlsCaFile(path string) GrpcReverseProxyOption {
	return func(opts *GrpcReverseProxyOptions) {
		opts.BackendTlsCaFile = path
	}
}

func WithBackendTlsVerifyCert(verifyCert bool) GrpcReverseProxyOption {
	return func(opts *GrpcReverseProxyOptions) {
		opts.BackendTlsVerifyCert = verifyCert
	}
}
