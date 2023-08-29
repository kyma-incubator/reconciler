package kubernetes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const EnvVarKubeconfig = "KUBECONFIG"

type ClientBuilder struct {
	logger     *zap.SugaredLogger
	kubeconfig []byte
	err        error
}

func NewClientBuilder() *ClientBuilder {
	return &ClientBuilder{}
}

func (cb *ClientBuilder) WithLogger(logger *zap.SugaredLogger) *ClientBuilder {
	cb.logger = logger
	return cb
}

func (cb *ClientBuilder) WithFile(filePath string) *ClientBuilder {
	cb.kubeconfig, cb.err = cb.loadFile(filePath)
	return cb
}

func (cb *ClientBuilder) WithString(kubeconfig string) *ClientBuilder {
	cb.kubeconfig = []byte(kubeconfig)
	return cb
}

func (cb *ClientBuilder) Build(ctx context.Context, validate bool) (kubernetes.Interface, error) {
	if cb.err != nil {
		return nil, cb.err
	}
	if len(cb.kubeconfig) == 0 {
		kubeconfigPath := os.Getenv(EnvVarKubeconfig)
		if kubeconfigPath == "" {
			return nil, fmt.Errorf("kubeconfig undefined: please provide it as file, string or set env-var %s",
				EnvVarKubeconfig)
		}
		kubeconfigPath, verifyErr := verifyPath(kubeconfigPath)
		if verifyErr != nil {
			return nil, errors.Wrap(verifyErr, fmt.Sprintf("potential path traversal detected for path %s", kubeconfigPath))
		}
		cb.kubeconfig, cb.err = cb.loadFile(kubeconfigPath)
	}
	config, err := clientcmd.RESTConfigFromKubeConfig(cb.kubeconfig)
	if err != nil {
		return nil, errors.Wrap(err,
			fmt.Sprintf("failed to create Kubernetes client configuration using provided kubeconfig: %s", cb.kubeconfig))
	}
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Kubernetes clientset by using provided REST-configuration")
	}

	if validate {
		start := time.Now()

		// wait for 0, 5, 15 (2*5+5), 35 (2*15+5) seconds total elapsed time
		backoff := wait.Backoff{
			Duration: 5 * time.Second,
			Factor:   2,
			Jitter:   0,
			Steps:    4,
			Cap:      35 * time.Second,
		}
		err = wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (done bool, err error) {
			_, err = clientSet.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
			if err != nil {
				cb.logger.Error(errors.Wrapf(err, "validation of connection to Kubernetes cluster %s failed (%.0f seconds elapsed since start), retrying", config.Host, time.Since(start).Seconds()))
				return false, nil
			}
			cb.logger.Debugf("validation of connection to Kubernetes cluster %s succeeded after %.0f seconds", config.Host, time.Since(start).Seconds())
			return true, nil
		})

		if err != nil {
			return nil, errors.Wrapf(err, "validation of connection to Kubernetes cluster %s failed after %.0f seconds", config.Host, time.Since(start).Seconds())
		}
	}

	return clientSet, err
}

func (cb *ClientBuilder) loadFile(filePath string) ([]byte, error) {
	if !file.Exists(filePath) {
		return nil, fmt.Errorf("kubeconfig file not found at path '%s'", filePath)
	}
	return os.ReadFile(filePath)
}

func verifyPath(path string) (string, error) {
	c := filepath.Clean(path)
	r, err := filepath.EvalSymlinks(c)
	if err != nil {
		return c, errors.Wrap(err, "Unsafe or invalid path specified")
	}
	return r, nil
}
