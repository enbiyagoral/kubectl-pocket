package cmd

import (
	"fmt"
	"os"

	"github.com/enbiyagoral/kubectl-pocket/pkg/k8s"
	"github.com/spf13/cobra"
)

var (
	// Version information
	Version   = "dev"
	GitCommit = "none"
	BuildDate = "unknown"

	// Global flags
	kubeconfig string
	namespace  string

	// K8s client (initialized lazily)
	k8sClient *k8s.Client
)

// GetK8sClient returns a Kubernetes client, creating one if needed
func GetK8sClient() (*k8s.Client, error) {
	if k8sClient != nil {
		return k8sClient, nil
	}
	var err error
	k8sClient, err = k8s.NewClient(kubeconfig, namespace)
	return k8sClient, err
}

var rootCmd = &cobra.Command{
	Use:   "kubectl-pocket",
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
  kubectl pocket pf redis 6379`,
	Version: fmt.Sprintf("%s (commit: %s, built: %s)", Version, GitCommit, BuildDate),
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "path to kubeconfig file")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "kubernetes namespace")
}
