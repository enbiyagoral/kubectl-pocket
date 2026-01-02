package cmd

import (
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test database connections from within the cluster",
	Long: `Test database connections by creating a temporary pod inside the cluster.

This command creates an ephemeral pod, runs the connection test, and then cleans up.
Useful for verifying database connectivity from within the Kubernetes network.

Supported databases:
  - MongoDB (mongo)
  - PostgreSQL (postgres)
  - Redis (redis)

Examples:
  kubectl pocket test mongo mongodb://mongo-svc:27017
  kubectl pocket test postgres postgres://user:pass@pg-svc:5432/mydb
  kubectl pocket test redis redis://redis-svc:6379`,
}

func init() {
	rootCmd.AddCommand(testCmd)
}
