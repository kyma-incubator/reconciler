package tests

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
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	envIstioIntegrationTests = "ISTIO_RECONCILER_INTEGRATION_TESTS"
)

type istioTest struct {
	logger        *zap.SugaredLogger
	kubeClient    reconcilerk8s.Client
	dynamicClient dynamic.Interface
	context       context.Context
	contextCancel context.CancelFunc
}

func newIstioTest(t *testing.T) *istioTest {
	log := logger.NewLogger(false)
	kubeconfig := test.ReadKubeconfig(t)
	kubeClient, err := reconcilerk8s.NewKubernetesClient(kubeconfig, log, &reconcilerk8s.Config{})
	require.NoError(t, err)
	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	require.NoError(t, err)
	dynamicClient, err := dynamic.NewForConfig(config)
	require.NoError(t, err)

	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)

	return &istioTest{
		logger:        log,
		kubeClient:    kubeClient,
		dynamicClient: dynamicClient,
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
	testEnabled, ok := os.LookupEnv(envIstioIntegrationTests)
	return ok && testEnabled == "1"
}
