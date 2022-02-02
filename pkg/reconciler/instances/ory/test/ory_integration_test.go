package test

import (
	"testing"

	"github.com/stretchr/testify/require"
	v1apps "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/util/podutils"
)

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

func TestOryIntegrationProduction(t *testing.T) {
	skipTestIfDisabled(t)

	if !isProductionProfile() {
		t.Skipf("Integration tests disabled: skipping parts of test case '%s'", t.Name())
	}

	setup := newOryTest(t)
	defer setup.contextCancel()

	t.Run("ensure that ory-hydra is deployed", func(t *testing.T) {
		name := "ory-hydra"
		hpa := setup.getHorizontalPodAutoscaler(t, name)

		require.GreaterOrEqual(t, int32(1), hpa.Status.CurrentReplicas)
		require.Equal(t, int32(3), hpa.Spec.MaxReplicas)
		setup.logger.Infof("HorizontalPodAutoscaler %v is deployed", hpa.Name)
	})

	t.Run("ensure that ory-oathkeeper is deployed", func(t *testing.T) {
		name := "ory-oathkeeper"
		hpa := setup.getHorizontalPodAutoscaler(t, name)

		require.GreaterOrEqual(t, int32(3), hpa.Status.CurrentReplicas)
		require.Equal(t, int32(10), hpa.Spec.MaxReplicas)
		setup.logger.Infof("HorizontalPodAutoscaler %v is deployed", hpa.Name)
	})

	t.Run("ensure that ory-postgresql is deployed", func(t *testing.T) {
		name := "ory-postgresql"
		sts := setup.getStatefulSet(t, name)

		require.Equal(t, int32(1), sts.Status.Replicas)
		require.Equal(t, int32(1), sts.Status.ReadyReplicas)
		setup.logger.Infof("StatefulSet %v is deployed", sts.Name)
	})

	t.Run("ensure that ory secrets are deployed", func(t *testing.T) {
		jwksName := "ory-oathkeeper-jwks-secret"
		credsName := "ory-hydra-credentials"

		setup.ensureSecretIsDeployed(t, jwksName)
		setup.ensureSecretIsDeployed(t, credsName)
	})
}

func (s *oryTest) getHorizontalPodAutoscaler(t *testing.T, name string) *autoscalingv1.HorizontalPodAutoscaler {
	hpa, err := s.kubeClient.AutoscalingV1().HorizontalPodAutoscalers(namespace).Get(s.context, name, metav1.GetOptions{})
	require.NoError(t, err)

	return hpa
}

func (s *oryTest) getStatefulSet(t *testing.T, name string) *v1apps.StatefulSet {
	sts, err := s.kubeClient.AppsV1().StatefulSets(namespace).Get(s.context, name, metav1.GetOptions{})
	require.NoError(t, err)

	return sts
}

func (s *oryTest) getSecret(t *testing.T, name string) *v1.Secret {
	secret, err := s.kubeClient.CoreV1().Secrets(namespace).Get(s.context, name, metav1.GetOptions{})
	require.NoError(t, err)

	return secret
}

func (s *oryTest) ensureSecretIsDeployed(t *testing.T, name string) {
	secret := s.getSecret(t, name)
	require.NotNil(t, secret.Data)
	s.logger.Infof("Secret %v is deployed", secret.Name)
}
