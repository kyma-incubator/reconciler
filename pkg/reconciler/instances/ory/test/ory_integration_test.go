package test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/util/podutils"
)

const (
	envOryIntegrationTests = "ORY_RECONCILER_INTEGRATION_TESTS"
	namespace              = "kyma-system"
)

func OryIntegrationTest(t *testing.T) {
	if !isIntegrationTestEnabled() {
		t.Skipf("Integration tests disabled: skipping parts of test case '%s'", t.Name())
	}
}

func isIntegrationTestEnabled() bool {
	expensiveTests, ok := os.LookupEnv(envOryIntegrationTests)
	return ok && (expensiveTests == "1" || strings.ToLower(expensiveTests) == "true")
}

func TestOryIntegration(t *testing.T) {
	OryIntegrationTest(t)

	log := logger.NewLogger(false)

	kubeClient, err := kubernetes.NewKubernetesClient(test.ReadKubeconfig(t), log, &kubernetes.Config{})
	require.NoError(t, err)

	client, err := kubeClient.Clientset()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	options := metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/instance=ory",
	}

	podsList, err := client.CoreV1().Pods(namespace).List(ctx, options)
	require.NoError(t, err)

	for i, pod := range podsList.Items {
		log.Infof("Pod %v is deployed", pod.Name)
		assert.Equal(t, v1.PodPhase("Running"), pod.Status.Phase)
		ready := podutils.IsPodAvailable(&podsList.Items[i], 0, metav1.Now())
		assert.Equal(t, true, ready)
	}
}
