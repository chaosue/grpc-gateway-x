package main

import (
	"github.com/spf13/viper"
	"time"
)

type Config struct {
	allowedOriginsMap map[string]struct{}
	// BindHost is the host to bind for serving, default is 0.0.0.0
	BindHost string `yaml:"bind_host"`
	// HttpPort is the TCP port to listen on for grpc-web, default is 8080. set it to 0 to disable grpc-web.
	HttpPort int `yaml:"http_port"`
	// GrpcPort is the TCP port to listen on for grpc, default is 8181. set it to 0 to disable grpc.
	GrpcPort      int
	EnableTls     bool
	TlsCertFile   string
	TlsKeyFile    string
	TlsCaFile     string
	TlsVerifyCert bool
	// ClientReadTimeout the timeout on reading data from client in ms. default is 10000 ms.
	ClientReadTimeout time.Duration
	// ClientWriteTimeout the timeout on sending data to client in ms. default is 10000 ms.
	ClientWriteTimeout time.Duration
	// GrpcMaxMessageSize maximum GRPC message size limit. If not specified, the default of 4MB will be used. (default 4194304)
	GrpcMaxMessageSize int
	// GracefulShutdownTimeout	default is 11000ms.
	GracefulShutdownTimeout time.Duration
	// Consul for backend service discovery.
	Consul struct {
		// Scheme if tls enabled, it should be set as "https", otherwise set it as "http", default is "http".
		Scheme string
		// TlsVerifyCert
		TlsVerifyCert bool
		// TlsCaFile the ca file that can be used to verify the peer's cert if TlsVerifyCert is enabled.
		TlsCaFile string
		// Addr the address of consul.
		Addr string
		//Token the consul authentication token.
		Token string
		// DC data-center of consul.
		DC string
	}
	// AllowAllOrigins whether allow requests from any origin. default is true.
	AllowAllOrigins bool
	// AllowedOrigins list of origin URLs which are allowed to make cross-origin requests.
	AllowedOrigins []string
	// AllowedHeaders list of headers which are allowed to propagate to the gRPC backend.
	AllowedHeaders []string
	// BackendAddress when explicitly set the grpc backend address/ip:port, the service auto-discovery via consul will be disabled.
	BackendAddress       string
	BackendEnableTls     bool
	BackendTlsVerifyCert bool
	BackendTlsCaFile     string
	EnableMetrics        bool
	EnableRequestTracing bool
}

func (c *Config) IsOriginAllowed(origin string) bool {
	if c.AllowAllOrigins {
		return true
	}
	if _, ok := c.allowedOriginsMap[origin]; ok {
		return true
	}
	return false
}

func (c *Config) Init() {
	if !c.AllowAllOrigins {
		c.allowedOriginsMap = map[string]struct{}{}
		for _, o := range c.AllowedOrigins {
			c.allowedOriginsMap[o] = struct{}{}
		}
	}
}

func init() {
	viper.SetDefault("BindHost", "0.0.0.0")
	viper.SetDefault("HttpPort", 8080)
	viper.SetDefault("GrpcPort", 8181)
	viper.SetDefault("AllowAllOrigins", true)
	viper.SetDefault("ClientReadTimeout", time.Second*10)
	viper.SetDefault("ClientWriteTimeout", time.Second*10)
	viper.SetDefault("GracefulShutdownTimeout", time.Second*11)
	viper.SetDefault("Consul.Scheme", "http")
	viper.SetDefault("GrpcMaxMessageSize", 4194304)
	viper.AddConfigPath(".")
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
}
