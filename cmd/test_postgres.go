package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/enbiyagoral/kubectl-pocket/pkg/k8s"
	"github.com/spf13/cobra"
)

var postgresCmd = &cobra.Command{
	Use:   "postgres <connection-string>",
	Short: "Test PostgreSQL connection",
	Long: `Test PostgreSQL connection from within the cluster.

Creates a temporary pod with psql client, tests the connection, and cleans up.

Examples:
  kubectl pocket test postgres postgres://pg-svc:5432/mydb
  kubectl pocket test postgres postgres://user:pass@pg-svc:5432/mydb
  kubectl pocket test postgres "postgres://pg-svc:5432/mydb?sslmode=disable"`,
	Args: cobra.ExactArgs(1),
	RunE: runPostgresTest,
}

var postgresTimeout time.Duration

func init() {
	testCmd.AddCommand(postgresCmd)
	postgresCmd.Flags().DurationVar(&postgresTimeout, "timeout", 30*time.Second, "connection test timeout")
}

func runPostgresTest(cmd *cobra.Command, args []string) error {
	connectionString := args[0]

	client, err := GetK8sClient()
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), postgresTimeout+30*time.Second)
	defer cancel()

	podName := fmt.Sprintf("pocket-postgres-test-%d", time.Now().Unix())
	ns := client.Namespace

	fmt.Printf("🔍 Testing PostgreSQL connection: %s\n", connectionString)
	fmt.Printf("📦 Creating test pod: %s/%s\n", ns, podName)

	// Create test pod
	podConfig := k8s.PodConfig{
		Name:      podName,
		Namespace: ns,
		Image:     "postgres:16-alpine",
		Command:   []string{"psql"},
		Args: []string{
			connectionString,
			"-c",
			"SELECT 1 as connection_test;",
		},
	}

	_, err = client.CreatePod(ctx, podConfig)
	if err != nil {
		return fmt.Errorf("failed to create pod: %w", err)
	}

	// Ensure cleanup
	defer func() {
		fmt.Printf("🧹 Cleaning up pod: %s\n", podName)
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = client.DeletePod(cleanupCtx, ns, podName)
	}()

	// Wait for completion
	fmt.Printf("⏳ Waiting for connection test...\n")
	pod, err := client.WaitForPodCompletion(ctx, ns, podName, postgresTimeout)
	if err != nil {
		return fmt.Errorf("timeout waiting for test: %w", err)
	}

	// Get logs
	logs, err := client.GetPodLogs(ctx, ns, podName)
	if err != nil {
		return fmt.Errorf("failed to get logs: %w", err)
	}

	if pod.Status.Phase == "Succeeded" {
		fmt.Printf("✅ PostgreSQL connection successful!\n")
		if logs != "" {
			fmt.Printf("📝 Output:\n%s\n", logs)
		}
		return nil
	}

	fmt.Printf("❌ PostgreSQL connection failed!\n")
	if logs != "" {
		fmt.Printf("📝 Error output:\n%s\n", logs)
	}
	return fmt.Errorf("connection test failed")
}
