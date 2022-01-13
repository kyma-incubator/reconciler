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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/util/podutils"
)

const (
	envOryIntegrationTests = "ORY_RECONCILER_INTEGRATION_TESTS"
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

func TestOryIntegration(t *testing.T) {
	skipTestIfDisabled(t)

	setup := newOryTest(t)
	defer setup.contextCancel()

	options := metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/instance=ory",
	}

	podsList, err := setup.kubeClient.CoreV1().Pods(namespace).List(setup.context, options)
	require.NoError(t, err)

	for i, pod := range podsList.Items {
		setup.logger.Infof("Pod %v is deployed", pod.Name)
		require.Equal(t, v1.PodPhase("Running"), pod.Status.Phase)
		ready := podutils.IsPodAvailable(&podsList.Items[i], 0, metav1.Now())
		require.Equal(t, true, ready)
	}
}
