package reverse_proxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/mwitkow/grpc-proxy/proxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	grpcReflection "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/grpc/status"
	"grpc-gateway-x/discovery"
	"os"
	"strings"
	"sync"
	"time"
)

const DefaultBackendConnPoolSize = 3

type BackendProxyDirector proxy.StreamDirector

type BackendDialer func(context.Context, ...kgrpc.ClientOption) (*grpc.ClientConn, error)

// EndpointParser parse endpoint part from grpc path, e.g.
//
// grpc requested service path is "/com.veigit.big-project.nice-app.grpc.v1.GoodService/Hello", then the endpoint registered should be parsed as "com.veigit.big-project.nice-app.grpc.v1"
type EndpointParser func(path string) (endpoint string, err error)
type GrpcReverseProxyOption func(opts *GrpcReverseProxyOptions)
type GrpcReverseProxyOptions struct {
	// EndpointParser parses endpoint from full method name.
	EndpointParser  EndpointParser
	BackendInsecure bool
	// BackendDiscovery is for resolving backend grpc server addresses of the endpoint.
	BackendDiscovery discovery.Discovery
	// BackendAddr the backend grpc server address. if BackendAddr is not set, the endpoint parsed from grpc request path will be used.
	BackendAddr string
	// BackendConnPoolSize maximum connections to each backend endpoint server
	BackendConnPoolSize  int
	BackendTlsCaFile     string
	BackendTlsVerifyCert bool
}
type BackendConnPool struct {
	sync.RWMutex
	conns *map[string]chan *grpc.ClientConn
}
type GrpcReverseProxy struct {
	opts            *GrpcReverseProxyOptions
	backendConnPool *BackendConnPool
	grpcReflection.UnimplementedServerReflectionServer
}

func NewReverseProxy(opts ...GrpcReverseProxyOption) (*GrpcReverseProxy, error) {
	grp := &GrpcReverseProxy{
		opts: &GrpcReverseProxyOptions{
			EndpointParser:      ParseEndpointFromGrpcRequestPath,
			BackendInsecure:     false,
			BackendConnPoolSize: DefaultBackendConnPoolSize,
		},
		backendConnPool: &BackendConnPool{
			conns: &map[string]chan *grpc.ClientConn{},
		},
	}
	for _, o := range opts {
		o(grp.opts)
	}
	if grp.opts.BackendAddr == "" && grp.opts.BackendDiscovery == nil {
		return nil, errors.New("none of BackendAddr or BackendDiscovery option is set")
	}
	return grp, nil
}

func (grp *GrpcReverseProxy) Director() BackendProxyDirector {
	return grp.streamDirector
}

func (grp *GrpcReverseProxy) DialBackend(ctx context.Context, endpoint string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	var conn *grpc.ClientConn
	var err error
	var backendCredential credentials.TransportCredentials
	if !grp.opts.BackendInsecure {
		tlsCfg := &tls.Config{
			InsecureSkipVerify: !grp.opts.BackendTlsVerifyCert,
		}
		if grp.opts.BackendTlsCaFile != "" {
			var cab []byte
			cab, err = os.ReadFile(grp.opts.BackendTlsCaFile)
			if err != nil {
				return nil, err
			}
			cp := x509.NewCertPool()
			if !cp.AppendCertsFromPEM(cab) {
				return nil, fmt.Errorf("credentials: failed to append ca certificates")
			}
			tlsCfg.RootCAs = cp
		}
		backendCredential = credentials.NewTLS(tlsCfg)
	} else {
		backendCredential = insecure.NewCredentials()
	}
	opts = append(opts, grpc.WithTransportCredentials(backendCredential), grpc.WithBlock())
	conn, err = grpc.DialContext(ctx, endpoint, opts...)
	return conn, err
}

func (grp *GrpcReverseProxy) streamDirector(ctx context.Context, serviceFullMethodName string) (context.Context, *grpc.ClientConn, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	outCtx, _ := context.WithCancel(ctx)
	mdCopy := md.Copy()
	delete(mdCopy, "user-agent")
	// If this header is present in the request from the web client,
	// the actual connection to the backend will not be established.
	// https://github.com/improbable-eng/grpc-web/issues/568
	delete(mdCopy, "connection")
	outCtx = metadata.NewOutgoingContext(outCtx, mdCopy)
	backendConn, err := grp.resolveServerConnection(serviceFullMethodName)
	if err != nil {
		return nil, nil, err
	}
	return outCtx, backendConn, nil
}

func (grp *GrpcReverseProxy) resolveServerConnection(serviceFullMethodName string) (conn *grpc.ClientConn, err error) {
	var endpoint string
	if grp.opts.BackendAddr != "" {
		endpoint = grp.opts.BackendAddr
	} else {
		endpoint, err = grp.opts.EndpointParser(serviceFullMethodName)
		if err != nil {
			return nil, err
		}
	}
	grp.backendConnPool.Lock()
	defer grp.backendConnPool.Unlock()
	conns, ok := (*grp.backendConnPool.conns)[endpoint]
	if !ok {
		conns = make(chan *grpc.ClientConn, grp.opts.BackendConnPoolSize)
		(*grp.backendConnPool.conns)[endpoint] = conns
	}
	select {
	case conn = <-conns:
		conns <- conn
	default:
	}
	if conn != nil && len(conns) >= grp.opts.BackendConnPoolSize {
		return conn, nil
	}
	var dialer BackendDialer
	if grp.opts.BackendInsecure {
		dialer = kgrpc.DialInsecure
	} else {
		dialer = kgrpc.Dial
	}
	dialOpts := []kgrpc.ClientOption{
		kgrpc.WithOptions(grpc.WithBlock()),
	}
	endpointWithScheme := "discovery:///" + endpoint
	// if backend address is explicitly specified, the address will be used and the service discovery will be ignored.
	if grp.opts.BackendAddr != "" {
		endpointWithScheme = endpoint
	} else {
		dialOpts = append(dialOpts, kgrpc.WithDiscovery(grp.opts.BackendDiscovery))
	}
	dialOpts = append(dialOpts, kgrpc.WithEndpoint(endpointWithScheme))
	ctx, cls := context.WithTimeout(context.TODO(), time.Second*2)
	defer cls()
	conn, err = dialer(ctx, dialOpts...)
	if err != nil {
		if context.DeadlineExceeded == err {
			err = status.New(codes.NotFound, "Resolving or dialing service timed out. this may be caused by invalid service name or unreachable backend server, service full method name: "+serviceFullMethodName).Err()
		}
		return nil, err
	}
	conns <- conn
	return
}

// ParseEndpointFromGrpcRequestPath the default endpoint parser for the reverse proxy. The rule is as below:
//
//	/{endpoint_registered}.{service}/{rpc}
//
// e.g.
//
//	/com.veigit.big-project.nice-app.grpc.v1.GoodService/Hello
//
// in case you have different path to endpoint and service/rpc rules, you can replace it as the code below :
//
//	NewReverseProxy(reverse_proxy.WithEndpointParser(YourOwnParser))
func ParseEndpointFromGrpcRequestPath(path string) (string, error) {
	if len(path) < 2 {
		return path, nil
	}
	if path[0] != '/' {
		return "", errors.New("grpc request path must start with '/'")
	}
	lp0 := strings.IndexByte(path[1:], '/')
	if lp0 < 0 {
		return "", errors.New("service name separator '/' not found in path")
	}
	lp1 := strings.LastIndexByte(path[1:lp0+1], '.')
	return path[1 : lp1+1], nil
}
