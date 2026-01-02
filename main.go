package main

import (
	"os"

	"github.com/enbiyagoral/kubectl-pocket/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
