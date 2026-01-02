package main

import (
	"os"

	"github.com/enbiyagoral/kubectl-pocket/cmd"
	_ "k8s.io/client-go/plugin/pkg/client/auth" // Import authentication plugins for cloud providers
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
