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

var mongoCmd = &cobra.Command{
	Use:   "mongo <connection-string>",
	Short: "Test MongoDB connection",
	Long: `Test MongoDB connection from within the cluster.

Creates a temporary pod with mongosh client, tests the connection, and cleans up.

Examples:
  kubectl pocket test mongo mongodb://mongo-svc:27017
  kubectl pocket test mongo mongodb://user:pass@mongo-svc:27017/mydb
  kubectl pocket test mongo mongodb://mongo-svc:27017 --shell`,
	Args: cobra.ExactArgs(1),
	RunE: runMongoTest,
}

var (
	mongoTimeout time.Duration
	mongoShell   bool
)

func init() {
	testCmd.AddCommand(mongoCmd)
	mongoCmd.Flags().DurationVar(&mongoTimeout, "timeout", 30*time.Second, "connection test timeout")
	mongoCmd.Flags().BoolVar(&mongoShell, "shell", false, "open interactive mongosh shell")
}

func runMongoTest(cmd *cobra.Command, args []string) error {
	connectionString := args[0]

	client, err := GetK8sClient()
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	podName := fmt.Sprintf("pocket-mongo-%d", time.Now().Unix())
	ns := client.Namespace

	// Shell mode - interactive mongosh
	if mongoShell {
		return runMongoShell(client, ns, podName, connectionString)
	}

	// Test mode
	ctx, cancel := context.WithTimeout(context.Background(), mongoTimeout+30*time.Second)
	defer cancel()

	fmt.Printf("🔍 Testing MongoDB connection: %s\n", connectionString)
	fmt.Printf("📦 Creating test pod: %s/%s\n", ns, podName)

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

	defer func() {
		fmt.Printf("🧹 Cleaning up pod: %s\n", podName)
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = client.DeletePod(cleanupCtx, ns, podName)
	}()

	fmt.Printf("⏳ Waiting for connection test...\n")
	pod, err := client.WaitForPodCompletion(ctx, ns, podName, mongoTimeout)
	if err != nil {
		return fmt.Errorf("timeout waiting for test: %w", err)
	}

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

func runMongoShell(client *k8s.Client, ns, podName, connStr string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	fmt.Printf("🚀 Starting MongoDB shell: %s\n", connStr)
	fmt.Printf("📦 Creating pod: %s/%s\n", ns, podName)

	podConfig := k8s.PodConfig{
		Name:      podName,
		Namespace: ns,
		Image:     "mongo:7",
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

	fmt.Printf("✅ Connected! Type 'exit' to quit.\n\n")

	// Set terminal to raw mode
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to set raw terminal: %w", err)
	}
	defer restoreTerminal(oldState)

	execOpts := k8s.ExecOptions{
		Namespace: ns,
		PodName:   podName,
		Container: "main",
		Command:   []string{"mongosh", connStr},
		Stdin:     os.Stdin,
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		TTY:       true,
	}

	return client.Exec(ctx, execOpts)
}

// restoreTerminal restores the terminal to its previous state
func restoreTerminal(oldState *term.State) {
	if err := term.Restore(int(os.Stdin.Fd()), oldState); err != nil {
		// Terminal may already be restored, ignore error
		_ = err
	}
}
