package k8s

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client wraps the Kubernetes clientset and config
type Client struct {
	Clientset  *kubernetes.Clientset
	Config     *rest.Config
	Namespace  string
	Kubeconfig string
}

// NewClient creates a new Kubernetes client
func NewClient(kubeconfig, namespace string) (*Client, error) {
	config, err := buildConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	// Get namespace from config if not specified
	if namespace == "" {
		namespace = getNamespaceFromConfig(kubeconfig)
	}

	return &Client{
		Clientset:  clientset,
		Config:     config,
		Namespace:  namespace,
		Kubeconfig: kubeconfig,
	}, nil
}

// buildConfig creates a rest.Config from kubeconfig or in-cluster config
func buildConfig(kubeconfig string) (*rest.Config, error) {
	// Try in-cluster config first if no kubeconfig specified
	if kubeconfig == "" {
		if config, err := rest.InClusterConfig(); err == nil {
			return config, nil
		}
		// Fall back to default kubeconfig location
		kubeconfig = defaultKubeconfigPath()
	}

	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

// defaultKubeconfigPath returns the default kubeconfig path
func defaultKubeconfigPath() string {
	if env := os.Getenv("KUBECONFIG"); env != "" {
		return env
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".kube", "config")
}

// getNamespaceFromConfig reads the current namespace from kubeconfig
func getNamespaceFromConfig(kubeconfig string) string {
	if kubeconfig == "" {
		kubeconfig = defaultKubeconfigPath()
	}

	config, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		return "default"
	}

	ctx, ok := config.Contexts[config.CurrentContext]
	if !ok || ctx.Namespace == "" {
		return "default"
	}

	return ctx.Namespace
}
