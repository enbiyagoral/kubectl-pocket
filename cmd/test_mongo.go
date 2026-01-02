package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/enbiyagoral/kubectl-pocket/pkg/k8s"
	"github.com/spf13/cobra"
)

var mongoCmd = &cobra.Command{
	Use:   "mongo <connection-string>",
	Short: "Test MongoDB connection",
	Long: `Test MongoDB connection from within the cluster.

Creates a temporary pod with mongosh client, tests the connection, and cleans up.

Examples:
  kubectl pocket test mongo mongodb://mongo-svc:27017
  kubectl pocket test mongo mongodb://user:pass@mongo-svc:27017/mydb
  kubectl pocket test mongo "mongodb://mongo-svc:27017/?replicaSet=rs0"`,
	Args: cobra.ExactArgs(1),
	RunE: runMongoTest,
}

var mongoTimeout time.Duration

func init() {
	testCmd.AddCommand(mongoCmd)
	mongoCmd.Flags().DurationVar(&mongoTimeout, "timeout", 30*time.Second, "connection test timeout")
}

func runMongoTest(cmd *cobra.Command, args []string) error {
	connectionString := args[0]

	client, err := GetK8sClient()
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), mongoTimeout+30*time.Second)
	defer cancel()

	podName := fmt.Sprintf("pocket-mongo-test-%d", time.Now().Unix())
	ns := client.Namespace

	fmt.Printf("🔍 Testing MongoDB connection: %s\n", connectionString)
	fmt.Printf("📦 Creating test pod: %s/%s\n", ns, podName)

	// Create test pod
	podConfig := k8s.PodConfig{
		Name:      podName,
		Namespace: ns,
		Image:     "mongo:7",
		Command:   []string{"mongosh"},
		Args: []string{
			connectionString,
			"--eval",
			"db.runCommand({ping: 1})",
			"--quiet",
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
	pod, err := client.WaitForPodCompletion(ctx, ns, podName, mongoTimeout)
	if err != nil {
		return fmt.Errorf("timeout waiting for test: %w", err)
	}

	// Get logs
	logs, err := client.GetPodLogs(ctx, ns, podName)
	if err != nil {
		return fmt.Errorf("failed to get logs: %w", err)
	}

	if pod.Status.Phase == "Succeeded" {
		fmt.Printf("✅ MongoDB connection successful!\n")
		if logs != "" {
			fmt.Printf("📝 Output:\n%s\n", logs)
		}
		return nil
	}

	fmt.Printf("❌ MongoDB connection failed!\n")
	if logs != "" {
		fmt.Printf("📝 Error output:\n%s\n", logs)
	}
	return fmt.Errorf("connection test failed")
}
