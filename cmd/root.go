package cmd

import (
	"fmt"
	"os"

	"github.com/enbiyagoral/kubectl-pocket/pkg/k8s"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
)

var (
	// Version information
	Version   = "dev"
	GitCommit = "none"
	BuildDate = "unknown"

	// Global config flags (cli-runtime)
	configFlags *genericclioptions.ConfigFlags

	// K8s client (initialized lazily)
	k8sClient *k8s.Client
)

// GetK8sClient returns a Kubernetes client, creating one if needed
func GetK8sClient() (*k8s.Client, error) {
	if k8sClient != nil {
		return k8sClient, nil
	}

	// Get kubeconfig path from configFlags
	kubeconfig := ""
	if configFlags != nil && configFlags.KubeConfig != nil {
		kubeconfig = *configFlags.KubeConfig
	}

	// Get namespace from configFlags
	namespace := ""
	if configFlags != nil && configFlags.Namespace != nil {
		namespace = *configFlags.Namespace
	}

	var err error
	k8sClient, err = k8s.NewClient(kubeconfig, namespace)
	return k8sClient, err
}

// NewRootCmd creates the root command
func NewRootCmd(streams genericiooptions.IOStreams) *cobra.Command {
	configFlags = genericclioptions.NewConfigFlags(true)

	rootCmd := &cobra.Command{
		Use:   "pocket",
		Short: "A Swiss Army knife for daily Kubernetes operations",
		Long: `kubectl-pocket is a Krew plugin that simplifies common DevOps tasks.

It provides quick access to:
  - Database connection testing (MongoDB, PostgreSQL, Redis)
  - Debug pods (busybox, netshoot)
  - Port-forward shortcuts

Examples:
  kubectl pocket test mongo mongodb://mongo-svc:27017
  kubectl pocket test postgres postgres://pg-svc:5432/mydb
  kubectl pocket debug netshoot
  kubectl pocket port-forward redis 6379`,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", Version, GitCommit, BuildDate),
	}

	// Add standard kubectl flags (--kubeconfig, --namespace, --context, --cluster, --user, etc.)
	configFlags.AddFlags(rootCmd.PersistentFlags())

	// Add subcommands
	addSubcommands(rootCmd)

	return rootCmd
}

// addSubcommands adds all subcommands to the root command
func addSubcommands(rootCmd *cobra.Command) {
	rootCmd.AddCommand(testCmd)
	rootCmd.AddCommand(pfCmd)
}

// Execute runs the root command
func Execute() error {
	streams := genericiooptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}
	rootCmd := NewRootCmd(streams)
	return rootCmd.Execute()
}
