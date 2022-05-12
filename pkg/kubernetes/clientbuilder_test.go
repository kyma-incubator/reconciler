package kubernetes

import (
	"context"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"testing"
)

func TestClientBuilderValidationTimeoutExponentialBackoff(t *testing.T) {
	t.Parallel()
	a := require.New(t)

	k8s, err := NewClientBuilder().
		WithFile("kubeconfig-unreachable.yaml").
		WithLogger(zaptest.NewLogger(t).Sugar()).
		Build(context.Background(), true)

	a.Nil(k8s, "Kubernetes interface should not be set")
	a.EqualError(err, "validation of connection to Kubernetes cluster https://0.0.0.0:12345 failed after 35 seconds: timed out waiting for the condition")
}
