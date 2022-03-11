package test

import (
	"testing"

	"github.com/avast/retry-go"
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

	podsList, err := setup.getPods(options)
	require.NoError(t, err)

	for i, pod := range podsList.Items {
		setup.logger.Infof("Pod %v is deployed", pod.Name)
		i := i
		pod := pod

		err := retry.Do(func() error {
			require.Equal(t, v1.PodPhase("Running"), pod.Status.Phase)
			ready := podutils.IsPodAvailable(&podsList.Items[i], 0, metav1.Now())
			require.Equal(t, true, ready)
			return nil
		}, retry.DelayType(retry.BackOffDelay))

		require.NoError(t, err)
	}

	t.Run("ensure that ory secrets are deployed", func(t *testing.T) {
		jwksName := "ory-oathkeeper-jwks-secret"
		credsName := "ory-hydra-credentials"
		setup.ensureSecretIsDeployed(t, jwksName)
		setup.ensureSecretIsDeployed(t, credsName)
	})
}

func TestOryIntegrationProduction(t *testing.T) {
	skipTestIfDisabled(t)

	if !isProductionProfile() {
		t.Skipf("Integration tests disabled: skipping parts of test case '%s'", t.Name())
	}

	setup := newOryTest(t)
	defer setup.contextCancel()

	t.Run("ensure that ory-hydra hpa is deployed", func(t *testing.T) {
		name := "ory-hydra"
		hpa, err := setup.getHorizontalPodAutoscaler(name)
		require.NoError(t, err)
		require.GreaterOrEqual(t, int32(1), hpa.Status.CurrentReplicas)
		require.Equal(t, int32(3), hpa.Spec.MaxReplicas)
		setup.logger.Infof("HorizontalPodAutoscaler %v is deployed", hpa.Name)
	})

	t.Run("ensure that ory-oathkeeper hpa is deployed", func(t *testing.T) {
		name := "ory-oathkeeper"
		hpa, err := setup.getHorizontalPodAutoscaler(name)
		require.NoError(t, err)
		require.GreaterOrEqual(t, int32(3), hpa.Status.CurrentReplicas)
		require.Equal(t, int32(10), hpa.Spec.MaxReplicas)
		setup.logger.Infof("HorizontalPodAutoscaler %v is deployed", hpa.Name)
	})

	t.Run("ensure that ory-postgresql is deployed", func(t *testing.T) {
		name := "ory-postgresql"
		sts, err := setup.getStatefulSet(t, name)
		require.NoError(t, err)

		require.Equal(t, int32(1), sts.Status.Replicas)
		require.Equal(t, int32(1), sts.Status.ReadyReplicas)
		setup.logger.Infof("StatefulSet %v is deployed", sts.Name)
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

	t.Run("ensure that single ory-hydra pod is deployed", func(t *testing.T) {
		options := metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/name=hydra",
			FieldSelector: "status.phase=Running",
		}
		podsList, err := setup.getPods(options)
		require.NoError(t, err)
		require.Equal(t, 1, len(podsList.Items))
		setup.logger.Infof("Single pod %v is deployed for app: hydra", podsList.Items[0].Name)
	})

	t.Run("ensure that single ory-hydra-maester pod is deployed", func(t *testing.T) {
		options := metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/name=hydra-maester",
			FieldSelector: "status.phase=Running",
		}
		err := retry.Do(func() error {
			podsList, err := setup.getPods(options)
			require.NoError(t, err)
			require.Equal(t, 1, len(podsList.Items))
			setup.logger.Infof("Single pod %v is deployed for app: hydra-maester", podsList.Items[0].Name)

			return nil
		}, retry.DelayType(retry.BackOffDelay))

		require.NoError(t, err)
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
