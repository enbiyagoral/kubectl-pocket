package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/enbiyagoral/kubectl-pocket/pkg/k8s"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// Supported databases only
var dbAliases = map[string]struct {
	serviceNames []string
	defaultPort  int
}{
	"redis":    {serviceNames: []string{"redis", "redis-master", "redis-svc"}, defaultPort: 6379},
	"mongo":    {serviceNames: []string{"mongo", "mongodb", "mongo-svc"}, defaultPort: 27017},
	"postgres": {serviceNames: []string{"postgres", "postgresql", "pg", "pg-svc"}, defaultPort: 5432},
}

var pfCmd = &cobra.Command{
	Use:     "pf <database> [local-port]",
	Aliases: []string{"portforward", "port-forward"},
	Short:   "Quick port-forward to database services",
	Long: `Quickly set up port-forwarding to supported database services.

Supported databases:
  - redis    : 6379
  - mongo    : 27017
  - postgres : 5432

Examples:
  kubectl pocket pf redis              # localhost:6379 -> redis:6379
  kubectl pocket pf redis 16379        # localhost:16379 -> redis:6379
  kubectl pocket pf mongo              # localhost:27017 -> mongo:27017
  kubectl pocket pf postgres 15432     # localhost:15432 -> postgres:5432`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runPortForward,
}

var pfAddress string

func init() {
	rootCmd.AddCommand(pfCmd)
	pfCmd.Flags().StringVar(&pfAddress, "address", "127.0.0.1", "local address to bind")
}

func runPortForward(cmd *cobra.Command, args []string) error {
	dbType := args[0]

	alias, ok := dbAliases[dbType]
	if !ok {
		return fmt.Errorf("unsupported database: %s (supported: redis, mongo, postgres)", dbType)
	}

	client, err := GetK8sClient()
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	ns := client.Namespace
	remotePort := alias.defaultPort
	localPort := alias.defaultPort

	// Parse optional local port
	if len(args) > 1 {
		localPort, err = strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid port: %s", args[1])
		}
	}

	// Find the service
	var serviceName string
	for _, name := range alias.serviceNames {
		_, err := client.Clientset.CoreV1().Services(ns).Get(context.Background(), name, metav1.GetOptions{})
		if err == nil {
			serviceName = name
			break
		}
	}

	if serviceName == "" {
		return fmt.Errorf("no %s service found in namespace %s (tried: %s)",
			dbType, ns, strings.Join(alias.serviceNames, ", "))
	}

	// Find pod for service
	podName, err := findPodForService(client, ns, serviceName)
	if err != nil {
		return fmt.Errorf("failed to find pod: %w", err)
	}

	// Build port-forward URL
	pfURL := client.Clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(ns).
		Name(podName).
		SubResource("portforward").
		URL()

	// Setup port-forward
	transport, upgrader, err := spdy.RoundTripperFor(client.Config)
	if err != nil {
		return fmt.Errorf("failed to create round tripper: %w", err)
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, pfURL)

	stopChan := make(chan struct{}, 1)
	readyChan := make(chan struct{})

	// Handle interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		close(stopChan)
	}()

	ports := []string{fmt.Sprintf("%d:%d", localPort, remotePort)}
	pf, err := portforward.New(dialer, ports, stopChan, readyChan, os.Stdout, os.Stderr)
	if err != nil {
		return fmt.Errorf("failed to create port-forwarder: %w", err)
	}

	fmt.Printf("🔌 Port-forwarding to %s\n", dbType)
	fmt.Printf("📡 %s:%d → %s:%d\n", pfAddress, localPort, serviceName, remotePort)
	fmt.Printf("💡 Press Ctrl+C to stop\n\n")

	return pf.ForwardPorts()
}

func findPodForService(client *k8s.Client, ns, serviceName string) (string, error) {
	svc, err := client.Clientset.CoreV1().Services(ns).Get(context.Background(), serviceName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	var selectors []string
	for k, v := range svc.Spec.Selector {
		selectors = append(selectors, fmt.Sprintf("%s=%s", k, v))
	}

	if len(selectors) == 0 {
		return "", fmt.Errorf("service has no selector")
	}

	pods, err := client.Clientset.CoreV1().Pods(ns).List(context.Background(), metav1.ListOptions{
		LabelSelector: strings.Join(selectors, ","),
		Limit:         1,
	})
	if err != nil {
		return "", err
	}

	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no pods found for service")
	}

	return pods.Items[0].Name, nil
}
