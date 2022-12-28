package main

import (
	"github.com/spf13/cobra"
	_ "net/http/pprof" // register in DefaultServerMux
)

func main() {
	rootCmd := cobra.Command{
		Use:     "grpc-gateway-x",
		Version: "0.0.1-alpha",
		Short:   "grpc-gateway-x is a proxy for clients accessing GRPC servers/services without need to know where the backend server are, and a reverse-proxy for GRPC servers to expose their services and response to the incoming requests.",
		RunE:    run,
	}
	rootCmd.Flags().String("config", "config.yaml", "the configuration file in yaml.")
	err := rootCmd.Execute()
	if err != nil {
		panic(err)
	}
}
