package test

import (
	"context"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
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
)

const (
	envOryIntegrationTests = "ORY_RECONCILER_INTEGRATION_TESTS_EVALUATION"
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

func TestOryIntegrationEvaluation(t *testing.T) {
	skipTestIfDisabled(t)

	setup := newOryTest(t)
	defer setup.contextCancel()

	// hydra-maester
	label := "app.kubernetes.io/name=hydra-maester"
	hpalist, err := getHpas(
		label,
		setup.kubeClient,
		setup.context,
		namespace,
	)
	require.NoError(t, err)
	require.Equal(t, 0, len(hpalist.Items))
	setup.logger.Infof("No hpa for app: %v is deployed", label)

	podsList, err := getPods(
		"app.kubernetes.io/name=hydra-maester",
		setup.kubeClient,
		setup.context,
		namespace,
	)
	require.NoError(t, err)
	require.Equal(t, 1, len(podsList.Items))
	pod := podsList.Items[0]
	setup.logger.Infof("Single pod %v is deployed for app: %v", pod.Name, label)

	// hydra
	label = "app.kubernetes.io/name=hydra"
	hpalist, err = getHpas(
		label,
		setup.kubeClient,
		setup.context,
		namespace,
	)
	require.NoError(t, err)
	require.Equal(t, 0, len(hpalist.Items))
	setup.logger.Infof("No hpa for app: %v is deployed", label)

	podsList, err = getPods(
		label,
		setup.kubeClient,
		setup.context,
		namespace,
	)
	require.NoError(t, err)
	require.Equal(t, 1, len(podsList.Items))
	pod = podsList.Items[0]
	setup.logger.Infof("Single pod %v is deployed for app: %v", pod.Name, label)

	// oathkeeper
	label = "app.kubernetes.io/name=oathkeeper"
	hpalist, err = getHpas(
		label,
		setup.kubeClient,
		setup.context,
		namespace,
	)
	require.NoError(t, err)
	require.Equal(t, 0, len(hpalist.Items))
	setup.logger.Infof("No hpa for app: %v is deployed", label)

	podsList, err = getPods(
		label,
		setup.kubeClient,
		setup.context,
		namespace,
	)
	require.NoError(t, err)
	require.Equal(t, 1, len(podsList.Items))
	pod = podsList.Items[0]
	setup.logger.Infof("Single pod %v is deployed for app: %v", pod.Name, label)
}

func getPods(label string, client kubernetes.Interface, ctx context.Context, namespace string) (*v1.PodList, error) {
	options := metav1.ListOptions{
		LabelSelector: label,
	}
	return client.CoreV1().Pods(namespace).List(ctx, options)
}
func getHpas(label string, client kubernetes.Interface, ctx context.Context, namespace string) (*autoscalingv1.HorizontalPodAutoscalerList, error) {
	options := metav1.ListOptions{
		LabelSelector: label,
	}
	return client.AutoscalingV1().HorizontalPodAutoscalers(namespace).List(ctx, options)
}
