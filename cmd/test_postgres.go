package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/enbiyagoral/kubectl-pocket/pkg/k8s"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var postgresCmd = &cobra.Command{
	Use:   "postgres <connection-string>",
	Short: "Test PostgreSQL connection",
	Long: `Test PostgreSQL connection from within the cluster.

Creates a temporary pod with psql client, tests the connection, and cleans up.

Examples:
  kubectl pocket test postgres postgres://pg-svc:5432/mydb
  kubectl pocket test postgres postgres://user:pass@pg-svc:5432/mydb
  kubectl pocket test postgres postgres://pg-svc:5432/mydb --shell`,
	Args: cobra.ExactArgs(1),
	RunE: runPostgresTest,
}

var (
	postgresTimeout time.Duration
	postgresShell   bool
)

func init() {
	testCmd.AddCommand(postgresCmd)
	postgresCmd.Flags().DurationVar(&postgresTimeout, "timeout", 30*time.Second, "connection test timeout")
	postgresCmd.Flags().BoolVar(&postgresShell, "shell", false, "open interactive psql shell")
}

func runPostgresTest(cmd *cobra.Command, args []string) error {
	connectionString := args[0]

	client, err := GetK8sClient()
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	podName := fmt.Sprintf("pocket-postgres-%d", time.Now().Unix())
	ns := client.Namespace

	// Shell mode
	if postgresShell {
		return runPostgresShell(client, ns, podName, connectionString)
	}

	// Test mode
	ctx, cancel := context.WithTimeout(context.Background(), postgresTimeout+30*time.Second)
	defer cancel()

	fmt.Printf("🔍 Testing PostgreSQL connection: %s\n", connectionString)
	fmt.Printf("📦 Creating test pod: %s/%s\n", ns, podName)

	podConfig := k8s.PodConfig{
		Name:      podName,
		Namespace: ns,
		Image:     "postgres:14-alpine",
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

	defer func() {
		fmt.Printf("🧹 Cleaning up pod: %s\n", podName)
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = client.DeletePod(cleanupCtx, ns, podName)
	}()

	fmt.Printf("⏳ Waiting for connection test...\n")
	pod, err := client.WaitForPodCompletion(ctx, ns, podName, postgresTimeout)
	if err != nil {
		return fmt.Errorf("timeout waiting for test: %w", err)
	}

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

func runPostgresShell(client *k8s.Client, ns, podName, connStr string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	fmt.Printf("🚀 Starting PostgreSQL shell: %s\n", connStr)
	fmt.Printf("📦 Creating pod: %s/%s\n", ns, podName)

	podConfig := k8s.PodConfig{
		Name:      podName,
		Namespace: ns,
		Image:     "postgres:14-alpine",
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

	fmt.Printf("✅ Connected! Type '\\q' to quit.\n\n")

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to set raw terminal: %w", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	execOpts := k8s.ExecOptions{
		Namespace: ns,
		PodName:   podName,
		Container: "main",
		Command:   []string{"psql", connStr},
		Stdin:     os.Stdin,
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		TTY:       true,
	}

	return client.Exec(ctx, execOpts)
}
