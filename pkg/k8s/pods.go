package k8s

import (
	"context"
	"fmt"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// PodConfig holds configuration for creating a pod
type PodConfig struct {
	Name      string
	Namespace string
	Image     string
	Command   []string
	Args      []string
	Env       []corev1.EnvVar
	TTY       bool
	Stdin     bool
}

// CreatePod creates a new pod with the given configuration
func (c *Client) CreatePod(ctx context.Context, config PodConfig) (*corev1.Pod, error) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.Name,
			Namespace: config.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "kubectl-pocket",
				"kubectl-pocket/temporary":     "true",
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:    "main",
					Image:   config.Image,
					Command: config.Command,
					Args:    config.Args,
					Env:     config.Env,
					TTY:     config.TTY,
					Stdin:   config.Stdin,
				},
			},
		},
	}

	return c.Clientset.CoreV1().Pods(config.Namespace).Create(ctx, pod, metav1.CreateOptions{})
}

// DeletePod deletes a pod by name
func (c *Client) DeletePod(ctx context.Context, namespace, name string) error {
	return c.Clientset.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// WaitForPodRunning waits until the pod is in Running state
func (c *Client) WaitForPodRunning(ctx context.Context, namespace, name string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		pod, err := c.Clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return pod.Status.Phase == corev1.PodRunning, nil
	})
}

// WaitForPodCompletion waits until the pod completes (Succeeded or Failed)
func (c *Client) WaitForPodCompletion(ctx context.Context, namespace, name string, timeout time.Duration) (*corev1.Pod, error) {
	var resultPod *corev1.Pod
	err := wait.PollUntilContextTimeout(ctx, time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		pod, err := c.Clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		resultPod = pod
		return pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed, nil
	})
	return resultPod, err
}

// GetPodLogs retrieves logs from a pod
func (c *Client) GetPodLogs(ctx context.Context, namespace, name string) (string, error) {
	req := c.Clientset.CoreV1().Pods(namespace).GetLogs(name, &corev1.PodLogOptions{})
	logs, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer closeStream(logs)

	buf, err := io.ReadAll(logs)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

// closeStream safely closes an io.ReadCloser, ignoring errors
func closeStream(closer io.Closer) {
	_ = closer.Close()
}

// ExecOptions holds options for executing a command in a pod
type ExecOptions struct {
	Namespace string
	PodName   string
	Container string
	Command   []string
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
	TTY       bool
}

// Exec executes a command in a pod
func (c *Client) Exec(ctx context.Context, opts ExecOptions) error {
	req := c.Clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(opts.PodName).
		Namespace(opts.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: opts.Container,
			Command:   opts.Command,
			Stdin:     opts.Stdin != nil,
			Stdout:    opts.Stdout != nil,
			Stderr:    opts.Stderr != nil,
			TTY:       opts.TTY,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(c.Config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}

	return exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  opts.Stdin,
		Stdout: opts.Stdout,
		Stderr: opts.Stderr,
		Tty:    opts.TTY,
	})
}
