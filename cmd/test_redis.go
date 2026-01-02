package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/enbiyagoral/kubectl-pocket/pkg/k8s"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var redisCmd = &cobra.Command{
	Use:   "redis <host:port>",
	Short: "Test Redis connection",
	Long: `Test Redis connection from within the cluster.

Creates a temporary pod with redis-cli, tests the connection, and cleans up.

Examples:
  kubectl pocket test redis redis-svc:6379
  kubectl pocket test redis redis://redis-svc:6379
  kubectl pocket test redis redis-svc:6379 --shell`,
	Args: cobra.ExactArgs(1),
	RunE: runRedisTest,
}

var (
	redisTimeout time.Duration
	redisShell   bool
)

func init() {
	testCmd.AddCommand(redisCmd)
	redisCmd.Flags().DurationVar(&redisTimeout, "timeout", 30*time.Second, "connection test timeout")
	redisCmd.Flags().BoolVar(&redisShell, "shell", false, "open interactive redis-cli shell")
}

func runRedisTest(cmd *cobra.Command, args []string) error {
	connectionString := args[0]

	client, err := GetK8sClient()
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	podName := fmt.Sprintf("pocket-redis-%d", time.Now().Unix())
	ns := client.Namespace

	// Parse connection string
	host, port, password := parseRedisConnection(connectionString)

	// Shell mode
	if redisShell {
		return runRedisShell(client, ns, podName, host, port, password)
	}

	// Test mode
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout+30*time.Second)
	defer cancel()

	fmt.Printf("🔍 Testing Redis connection: %s:%s\n", host, port)
	fmt.Printf("📦 Creating test pod: %s/%s\n", ns, podName)

	redisArgs := []string{"-h", host, "-p", port, "PING"}
	if password != "" {
		redisArgs = []string{"-h", host, "-p", port, "-a", password, "PING"}
	}

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

	defer func() {
		fmt.Printf("🧹 Cleaning up pod: %s\n", podName)
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = client.DeletePod(cleanupCtx, ns, podName)
	}()

	fmt.Printf("⏳ Waiting for connection test...\n")
	pod, err := client.WaitForPodCompletion(ctx, ns, podName, redisTimeout)
	if err != nil {
		return fmt.Errorf("timeout waiting for test: %w", err)
	}

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

func runRedisShell(client *k8s.Client, ns, podName, host, port, password string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	fmt.Printf("🚀 Starting Redis shell: %s:%s\n", host, port)
	fmt.Printf("📦 Creating pod: %s/%s\n", ns, podName)

	podConfig := k8s.PodConfig{
		Name:      podName,
		Namespace: ns,
		Image:     "redis:7-alpine",
		Command:   []string{"sleep", "3600"},
		TTY:       true,
		Stdin:     true,
	}

	_, err := client.CreatePod(ctx, podConfig)
	if err != nil {
		return fmt.Errorf("failed to create pod: %w", err)
	}

	defer func() {
		fmt.Printf("\n🧹 Cleaning up pod: %s\n", podName)
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = client.DeletePod(cleanupCtx, ns, podName)
	}()

	fmt.Printf("⏳ Waiting for pod to be ready...\n")
	if err := client.WaitForPodRunning(ctx, ns, podName, 2*time.Minute); err != nil {
		return fmt.Errorf("pod failed to start: %w", err)
	}

	fmt.Printf("✅ Connected! Type 'quit' to exit.\n\n")

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to set raw terminal: %w", err)
	}
	defer restoreTerminal(oldState)

	// Build redis-cli command
	redisCliCmd := []string{"redis-cli", "-h", host, "-p", port}
	if password != "" {
		redisCliCmd = append(redisCliCmd, "-a", password)
	}

	execOpts := k8s.ExecOptions{
		Namespace: ns,
		PodName:   podName,
		Container: "main",
		Command:   redisCliCmd,
		Stdin:     os.Stdin,
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		TTY:       true,
	}

	return client.Exec(ctx, execOpts)
}

// parseRedisConnection parses various Redis connection formats
func parseRedisConnection(conn string) (host, port, password string) {
	port = "6379"
	conn = strings.TrimPrefix(conn, "redis://")

	if strings.Contains(conn, "@") {
		parts := strings.SplitN(conn, "@", 2)
		password = strings.TrimPrefix(parts[0], ":")
		conn = parts[1]
	}

	if strings.Contains(conn, ":") {
		parts := strings.SplitN(conn, ":", 2)
		host = parts[0]
		port = parts[1]
	} else {
		host = conn
	}

	return host, port, password
}
