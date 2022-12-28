package main

import (
	"crypto/tls"
	"os"

	"crypto/x509"
	"github.com/mwitkow/go-conntrack/connhelpers"
	logrus "github.com/sirupsen/logrus"
)

func buildServerTlsOrFail(cfg *Config) *tls.Config {
	if cfg.TlsCertFile == "" || cfg.TlsKeyFile == "" {
		logrus.Fatalf("TlsCertFile and TlsKeyFile must be set")
	}
	tlsConfig, err := connhelpers.TlsConfigForServerCerts(cfg.TlsCertFile, cfg.TlsKeyFile)
	if err != nil {
		logrus.Fatalf("failed reading TLS server keys: %v", err)
	}
	tlsConfig.MinVersion = tls.VersionTLS12
	switch cfg.TlsVerifyCert {
	case false:
		tlsConfig.ClientAuth = tls.NoClientCert
	default:
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	if tlsConfig.ClientAuth != tls.NoClientCert {
		if cfg.TlsCaFile != "" {
			tlsConfig.ClientCAs = x509.NewCertPool()
			data, err := os.ReadFile(cfg.TlsCaFile)
			if err != nil {
				logrus.Fatalf("failed reading client CA file %v: %v", cfg.TlsCaFile, err)
			}
			if ok := tlsConfig.ClientCAs.AppendCertsFromPEM(data); !ok {
				logrus.Fatalf("failed processing client CA file %v", cfg.TlsCaFile)
			}
		} else {
			var err error
			tlsConfig.ClientCAs, err = x509.SystemCertPool()
			if err != nil {
				logrus.Fatalf("no client CA files specified, fallback to system CA chain failed: %v", err)
			}
		}

	}
	tlsConfig, err = connhelpers.TlsConfigWithHttp2Enabled(tlsConfig)
	if err != nil {
		logrus.Fatalf("can't configure h2 handling: %v", err)
	}
	return tlsConfig
}
