package test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	reconcilerk8s "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
)

const (
	envOryIntegrationTests = "ORY_RECONCILER_INTEGRATION_TESTS"
	envExecutionProfile    = "EXECUTION_PROFILE"
	namespace              = "kyma-system"
)

type oryTest struct {
	logger        *zap.SugaredLogger
	kubeClient    kubernetes.Interface
	context       context.Context
	contextCancel context.CancelFunc
}

func newOryTest(t *testing.T) *oryTest {
	log := logger.NewLogger(false)
	kubeClient, err := reconcilerk8s.NewKubernetesClient(test.ReadKubeconfig(t), log, &reconcilerk8s.Config{})
	require.NoError(t, err)

	client, err := kubeClient.Clientset()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)

	return &oryTest{
		logger:        log,
		kubeClient:    client,
		context:       ctx,
		contextCancel: cancel,
	}
}

func skipTestIfDisabled(t *testing.T) {
	if !isIntegrationTestEnabled() {
		t.Skipf("Integration tests disabled: skipping parts of test case '%s'", t.Name())
	}
}

func isIntegrationTestEnabled() bool {
	testEnabled, ok := os.LookupEnv(envOryIntegrationTests)
	return ok && testEnabled == "1"
}

func isProductionProfile() bool {
	prodEnabled, ok := os.LookupEnv(envExecutionProfile)

	return ok && prodEnabled == "production"
}
