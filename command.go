package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_logrus "github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/hashicorp/consul/api"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/mwitkow/go-conntrack"
	"github.com/mwitkow/grpc-proxy/proxy"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/net/trace"
	"google.golang.org/grpc"
	grpcReflection "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"vcs.intra.veigit.com/infra/grpc-gateway-x/discovery"
	reverse_proxy "vcs.intra.veigit.com/infra/grpc-gateway-x/reverse-proxy"
)

func run(cmd *cobra.Command, _ []string) error {
	logrus.SetOutput(os.Stdout)
	logrus.SetReportCaller(false)
	l := logrus.StandardLogger()
	l.SetLevel(logrus.TraceLevel)
	logEntry := logrus.NewEntry(l)
	configFile, _ := cmd.Flags().GetString("config")
	if configFile != "" {
		viper.SetConfigFile(configFile)
	}
	err := viper.ReadInConfig()
	if err != nil {
		return err
	}
	cfg := &Config{}
	err = viper.Unmarshal(cfg)
	if err != nil {
		return err
	}
	if cfg.AllowAllOrigins && len(cfg.AllowedOrigins) != 0 {
		return errors.New("ambiguous AllowAllOrigins and AllowedOrigins configuration. Either set AllowAllOrigins to true OR specify one or more origins to whitelist in AllowedOrigins, not both")
	}
	cfg.Init()

	errChan := make(chan error, 3)
	var servingHttpServer *http.Server
	if cfg.HttpPort > 0 {
		grpcServerForWeb := buildGrpcProxyServer(logEntry, cfg)
		options := []grpcweb.Option{
			grpcweb.WithCorsForRegisteredEndpointsOnly(false),
			grpcweb.WithOriginFunc(cfg.IsOriginAllowed),
		}

		if len(cfg.AllowedHeaders) > 0 {
			options = append(
				options,
				grpcweb.WithAllowedRequestHeaders(cfg.AllowedHeaders),
			)
		}

		wrappedGrpc := grpcweb.WrapServer(grpcServerForWeb, options...)

		serveMux := http.NewServeMux()
		serveMux.Handle("/", wrappedGrpc)
		if cfg.EnableMetrics {
			serveMux.Handle("/metrics", promhttp.Handler())
		}
		if cfg.EnableRequestTracing {
			serveMux.HandleFunc("/debug/requests", func(resp http.ResponseWriter, req *http.Request) {
				trace.Traces(resp, req)
			})
			serveMux.HandleFunc("/debug/events", func(resp http.ResponseWriter, req *http.Request) {
				trace.Events(resp, req)
			})
		}
		servingHttpServer = buildServer(serveMux, cfg)
		servingListener := buildListenerOrFail("grpc-web", cfg.BindHost, cfg.HttpPort)
		if cfg.EnableTls {
			servingListener = tls.NewListener(servingListener, buildServerTlsOrFail(cfg))
		}
		serveGrpcWebServer(servingHttpServer, servingListener, errChan)
	}

	var grpcServer *grpc.Server
	if cfg.GrpcPort > 0 {
		grpcServer := buildGrpcProxyServer(logEntry, cfg)
		grpcServingListener := buildListenerOrFail("grpc", cfg.BindHost, cfg.GrpcPort)
		if cfg.EnableTls {
			grpcServingListener = tls.NewListener(grpcServingListener, buildServerTlsOrFail(cfg))
		}
		serveGrpcServer(grpcServer, grpcServingListener, errChan)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sigChan:
		if grpcServer != nil {
			grpcServer.GracefulStop()
		}
		ctx, cls := context.WithTimeout(context.Background(), cfg.GracefulShutdownTimeout)
		defer cls()
		if servingHttpServer != nil {
			err = servingHttpServer.Shutdown(ctx)
			errChan <- err
		}
	}
ErrH:
	for {
		select {
		case <-time.After(time.Millisecond * 10):
			break ErrH
		case err = <-errChan:
			if err != nil && strings.LastIndex(err.Error(), http.ErrServerClosed.Error()) == -1 {
				logrus.Warningf("errors on serving: %v", err)
			}
		}
	}
	return nil
}

func buildServer(handler http.Handler, cfg *Config) *http.Server {
	return &http.Server{
		WriteTimeout: cfg.ClientWriteTimeout * time.Millisecond,
		ReadTimeout:  cfg.ClientReadTimeout * time.Millisecond,
		Handler:      handler,
	}
}

func serveGrpcWebServer(server *http.Server, listener net.Listener, errChan chan error) {
	go func() {
		logrus.Infof("serving grpc-web on: %v", listener.Addr().String())
		if err := server.Serve(listener); err != nil {
			errChan <- fmt.Errorf("serve error: %v", err)
		}
	}()
}

func serveGrpcServer(server *grpc.Server, listener net.Listener, errChan chan error) {
	go func() {
		logrus.Infof("serving grpc on: %v", listener.Addr().String())
		if err := server.Serve(listener); err != nil {
			errChan <- fmt.Errorf("serve error: %v", err)
		}
	}()
}
func buildGrpcProxyServer(logger *logrus.Entry, cfg *Config) *grpc.Server {
	grpc.EnableTracing = true
	grpc_logrus.ReplaceGrpcLogger(logger)
	consulConfig := &api.Config{
		Address: cfg.Consul.Addr,
		Token:   cfg.Consul.Token,
		Scheme:  cfg.Consul.Scheme,
	}
	if cfg.Consul.Scheme == "https" {
		consulConfig.TLSConfig = api.TLSConfig{}
		if !cfg.Consul.TlsVerifyCert {
			consulConfig.TLSConfig.InsecureSkipVerify = true
		}
		if cfg.Consul.TlsCaFile != "" {
			consulConfig.TLSConfig.CAFile = cfg.Consul.TlsCaFile
		}
	}
	d := discovery.NewConsul(consulConfig)
	rp, err := reverse_proxy.NewReverseProxy(
		reverse_proxy.WithBackendAddr(cfg.BackendAddress),
		reverse_proxy.WithBackendInsecure(!cfg.BackendEnableTls),
		reverse_proxy.WithBackendDiscovery(d),
		reverse_proxy.WithBackendTlsVerifyCert(cfg.BackendTlsVerifyCert),
		reverse_proxy.WithBackendTlsCaFile(cfg.BackendTlsCaFile),
	)
	if err != nil {
		panic(err)
	}

	// Server with logging and monitoring enabled.
	srv := grpc.NewServer(
		grpc.UnknownServiceHandler(proxy.TransparentHandler(proxy.StreamDirector(rp.Director()))),
		grpc.MaxRecvMsgSize(cfg.GrpcMaxMessageSize),
		grpc_middleware.WithUnaryServerChain(
			grpc_logrus.UnaryServerInterceptor(logger),
			grpc_prometheus.UnaryServerInterceptor,
		),
		grpc_middleware.WithStreamServerChain(
			grpc_logrus.StreamServerInterceptor(logger),
			grpc_prometheus.StreamServerInterceptor,
		),
	)
	grpcReflection.RegisterServerReflectionServer(srv, rp)
	return srv
}

func buildListenerOrFail(name string, host string, port int) net.Listener {
	addr := fmt.Sprintf("%s:%d", host, port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		logrus.Fatalf("failed listening for '%v' on %v: %v", name, addr, err)
	}
	return conntrack.NewListener(listener,
		conntrack.TrackWithName(name),
		conntrack.TrackWithTcpKeepAlive(20*time.Second),
		conntrack.TrackWithTracing(),
	)
}
