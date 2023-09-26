package test

import (
	"github.com/stretchr/testify/require"
	v1apps "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestOryIntegration(t *testing.T) {
	skipTestIfDisabled(t)

	setup := newOryTest(t)
	defer setup.contextCancel()

	t.Run("ensure that ory pods are not deployed", func(t *testing.T) {
		options := metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/instance=ory",
		}

		podsList, err := setup.getPods(options)
		require.NoError(t, err)
		require.Empty(t, podsList.Items)
	})

	t.Run("ensure that ory secrets are deployed", func(t *testing.T) {
		jwksName := "ory-oathkeeper-jwks-secret"
		setup.ensureSecretIsDeployed(t, jwksName)
	})
}

func TestOryIntegrationProduction(t *testing.T) {
	skipTestIfDisabled(t)

	if !isProductionProfile() {
		t.Skipf("Integration tests disabled: skipping parts of test case '%s'", t.Name())
	}

	setup := newOryTest(t)
	defer setup.contextCancel()

	t.Run("ensure that ory-hydra hpa is not deployed", func(t *testing.T) {
		name := "ory-hydra"
		_, err := setup.getHorizontalPodAutoscaler(name)
		require.Error(t, err)
		setup.logger.Infof("HorizontalPodAutoscaler is not deployed")
	})

	t.Run("ensure that ory-oathkeeper hpa is deployed", func(t *testing.T) {
		name := "ory-oathkeeper"
		hpa, err := setup.getHorizontalPodAutoscaler(name)
		require.NoError(t, err)
		require.GreaterOrEqual(t, int32(3), hpa.Status.CurrentReplicas)
		require.Equal(t, int32(10), hpa.Spec.MaxReplicas)
		setup.logger.Infof("HorizontalPodAutoscaler %v is deployed", hpa.Name)
	})

	t.Run("ensure that ory-postgresql is not deployed", func(t *testing.T) {
		name := "ory-postgresql"
		_, err := setup.getStatefulSet(name)
		require.Error(t, err)
		require.True(t, kerrors.IsNotFound(err))
	})

	t.Run("ensure that ory-hydra pod is not deployed", func(t *testing.T) {
		options := metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/name=hydra",
		}
		podsList, err := setup.getPods(options)
		require.NoError(t, err)
		require.Empty(t, podsList.Items)
	})

}

func TestOryIntegrationEvaluation(t *testing.T) {
	skipTestIfDisabled(t)

	if !isEvaluationProfile() {
		t.Skipf("Integration tests disabled: skipping parts of test case '%s'", t.Name())
	}

	setup := newOryTest(t)
	defer setup.contextCancel()

	t.Run("ensure that ory-hydra hpa is not deployed", func(t *testing.T) {
		name := "ory-hydra"
		_, err := setup.getHorizontalPodAutoscaler(name)
		require.Error(t, err)
		setup.logger.Infof("HorizontalPodAutoscaler is not deployed")
	})

	t.Run("ensure that ory-oathkeeper hpa is not deployed", func(t *testing.T) {
		name := "ory-oathkeeper"
		_, err := setup.getHorizontalPodAutoscaler(name)
		require.Error(t, err)
		setup.logger.Infof("HorizontalPodAutoscaler is not deployed")
	})

	t.Run("ensure that single ory-oathkeeper pod is deployed", func(t *testing.T) {
		options := metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/name=oathkeeper",
			FieldSelector: "status.phase=Running",
		}
		podsList, err := setup.getPods(options)
		require.NoError(t, err)
		require.Equal(t, 1, len(podsList.Items))
		setup.logger.Infof("Single pod %v is deployed for app: oathkeeper", podsList.Items[0].Name)
	})

	t.Run("ensure that ory-hydra pod is not deployed", func(t *testing.T) {
		options := metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/name=hydra",
		}
		podsList, err := setup.getPods(options)
		require.NoError(t, err)
		require.Empty(t, podsList.Items)
	})
}

func (s *oryTest) getPods(options metav1.ListOptions) (*v1.PodList, error) {
	return s.kubeClient.CoreV1().Pods(namespace).List(s.context, options)
}

func (s *oryTest) getHorizontalPodAutoscaler(name string) (*autoscalingv1.HorizontalPodAutoscaler, error) {
	return s.kubeClient.AutoscalingV1().HorizontalPodAutoscalers(namespace).Get(s.context, name, metav1.GetOptions{})
}

func (s *oryTest) getStatefulSet(name string) (*v1apps.StatefulSet, error) {
	return s.kubeClient.AppsV1().StatefulSets(namespace).Get(s.context, name, metav1.GetOptions{})
}

func (s *oryTest) getSecret(name string) (*v1.Secret, error) {
	return s.kubeClient.CoreV1().Secrets(namespace).Get(s.context, name, metav1.GetOptions{})
}

func (s *oryTest) ensureSecretIsDeployed(t *testing.T, name string) {
	secret, err := s.getSecret(name)
	require.NoError(t, err)
	require.NotNil(t, secret.Data)
	s.logger.Infof("Secret %v is deployed", secret.Name)
}
