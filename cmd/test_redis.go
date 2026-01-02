package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/enbiyagoral/kubectl-pocket/pkg/k8s"
	"github.com/spf13/cobra"
)

var redisCmd = &cobra.Command{
	Use:   "redis <host:port>",
	Short: "Test Redis connection",
	Long: `Test Redis connection from within the cluster.

Creates a temporary pod with redis-cli, tests the connection, and cleans up.

Examples:
  kubectl pocket test redis redis-svc:6379
  kubectl pocket test redis redis://redis-svc:6379
  kubectl pocket test redis redis://:password@redis-svc:6379`,
	Args: cobra.ExactArgs(1),
	RunE: runRedisTest,
}

var redisTimeout time.Duration

func init() {
	testCmd.AddCommand(redisCmd)
	redisCmd.Flags().DurationVar(&redisTimeout, "timeout", 30*time.Second, "connection test timeout")
}

func runRedisTest(cmd *cobra.Command, args []string) error {
	connectionString := args[0]

	client, err := GetK8sClient()
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout+30*time.Second)
	defer cancel()

	podName := fmt.Sprintf("pocket-redis-test-%d", time.Now().Unix())
	ns := client.Namespace

	// Parse connection string
	host, port, password := parseRedisConnection(connectionString)

	fmt.Printf("🔍 Testing Redis connection: %s:%s\n", host, port)
	fmt.Printf("📦 Creating test pod: %s/%s\n", ns, podName)

	// Build redis-cli arguments
	redisArgs := []string{"-h", host, "-p", port, "PING"}
	if password != "" {
		redisArgs = []string{"-h", host, "-p", port, "-a", password, "PING"}
	}

	// Create test pod
	podConfig := k8s.PodConfig{
		Name:      podName,
		Namespace: ns,
		Image:     "redis:7-alpine",
		Command:   []string{"redis-cli"},
		Args:      redisArgs,
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
	pod, err := client.WaitForPodCompletion(ctx, ns, podName, redisTimeout)
	if err != nil {
		return fmt.Errorf("timeout waiting for test: %w", err)
	}

	// Get logs
	logs, err := client.GetPodLogs(ctx, ns, podName)
	if err != nil {
		return fmt.Errorf("failed to get logs: %w", err)
	}

	logs = strings.TrimSpace(logs)

	if pod.Status.Phase == "Succeeded" && strings.Contains(logs, "PONG") {
		fmt.Printf("✅ Redis connection successful!\n")
		fmt.Printf("📝 Response: %s\n", logs)
		return nil
	}

	fmt.Printf("❌ Redis connection failed!\n")
	if logs != "" {
		fmt.Printf("📝 Error output:\n%s\n", logs)
	}
	return fmt.Errorf("connection test failed")
}

// parseRedisConnection parses various Redis connection formats
// Supports: host:port, redis://host:port, redis://:password@host:port
func parseRedisConnection(conn string) (host, port, password string) {
	port = "6379" // default

	// Remove redis:// prefix if present
	conn = strings.TrimPrefix(conn, "redis://")

	// Check for password
	if strings.Contains(conn, "@") {
		parts := strings.SplitN(conn, "@", 2)
		password = strings.TrimPrefix(parts[0], ":")
		conn = parts[1]
	}

	// Parse host:port
	if strings.Contains(conn, ":") {
		parts := strings.SplitN(conn, ":", 2)
		host = parts[0]
		port = parts[1]
	} else {
		host = conn
	}

	return host, port, password
}
