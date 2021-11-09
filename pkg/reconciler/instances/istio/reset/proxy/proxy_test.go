package proxy

import (
	"errors"
	"testing"

	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/config"
	datamocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/data/mocks"
	podresetmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/pod/reset/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
)

func Test_IstioProxyReset_Run(t *testing.T) {
	cfg := config.IstioProxyConfig{
		ImagePrefix:  "istio/proxyv2",
		ImageVersion: "1.10.2",
		RetriesCount: 5,
		Kubeclient:   fake.NewSimpleClientset(),
		Log:          log.NewLogger(true),
	}

	t.Run("should not return an error when no pods are present on the cluster", func(t *testing.T) {
		// given
		gatherer := datamocks.Gatherer{}
		gatherer.On("GetAllPods", mock.Anything, mock.AnythingOfType("[]retry.Option")).Return(&v1.PodList{}, nil)
		gatherer.On("GetPodsWithDifferentImage", mock.AnythingOfType("v1.PodList"),
			mock.AnythingOfType("data.ExpectedImage")).Return(v1.PodList{})

		action := podresetmocks.Action{}
		action.On("Reset", mock.Anything, mock.AnythingOfType("[]retry.Option"), mock.AnythingOfType("v1.PodList"), mock.AnythingOfType("*zap.SugaredLogger"), mock.AnythingOfType("bool"), mock.AnythingOfType("pod.WaitOptions")).
			Return(nil)
		istioProxyReset := NewDefaultIstioProxyReset(&gatherer, &action)

		// when
		err := istioProxyReset.Run(cfg)

		// then
		require.NoError(t, err)
		gatherer.AssertNumberOfCalls(t, "GetAllPods", 1)
		gatherer.AssertNumberOfCalls(t, "GetPodsWithDifferentImage", 1)
		action.AssertNumberOfCalls(t, "Reset", 1)
	})

	t.Run("should not return an error when pods are present on the cluster", func(t *testing.T) {
		// given
		gatherer := datamocks.Gatherer{}
		gatherer.On("GetAllPods", mock.Anything, mock.AnythingOfType("[]retry.Option")).Return(&v1.PodList{Items: []v1.Pod{{}}}, nil)
		gatherer.On("GetPodsWithDifferentImage", mock.AnythingOfType("v1.PodList"),
			mock.AnythingOfType("data.ExpectedImage")).Return(v1.PodList{Items: []v1.Pod{{}}})

		action := podresetmocks.Action{}
		action.On("Reset", mock.Anything, mock.AnythingOfType("[]retry.Option"), mock.AnythingOfType("v1.PodList"), mock.AnythingOfType("*zap.SugaredLogger"), mock.AnythingOfType("bool"), mock.AnythingOfType("pod.WaitOptions")).
			Return(nil)
		istioProxyReset := NewDefaultIstioProxyReset(&gatherer, &action)

		// when
		err := istioProxyReset.Run(cfg)

		// then
		require.NoError(t, err)
		gatherer.AssertNumberOfCalls(t, "GetAllPods", 1)
		gatherer.AssertNumberOfCalls(t, "GetPodsWithDifferentImage", 1)
		action.AssertNumberOfCalls(t, "Reset", 1)
	})

	t.Run("should return an error when GetAllPods returns an error", func(t *testing.T) {
		// given
		expectedError := errors.New("GetAllPods error")
		gatherer := datamocks.Gatherer{}
		gatherer.On("GetAllPods", mock.Anything, mock.AnythingOfType("[]retry.Option")).Return(nil, expectedError)
		gatherer.On("GetPodsWithDifferentImage", mock.AnythingOfType("v1.PodList"),
			mock.AnythingOfType("data.ExpectedImage")).Return(v1.PodList{})

		action := podresetmocks.Action{}
		action.On("Reset", mock.Anything, mock.AnythingOfType("[]retry.Option"), mock.AnythingOfType("v1.PodList"), mock.AnythingOfType("*zap.SugaredLogger"), mock.AnythingOfType("bool"), mock.AnythingOfType("pod.WaitOptions")).
			Return(nil)
		istioProxyReset := DefaultIstioProxyReset{&gatherer, &action}

		// when
		err := istioProxyReset.Run(cfg)

		// then
		require.ErrorIs(t, err, expectedError)
		gatherer.AssertNumberOfCalls(t, "GetAllPods", 1)
		gatherer.AssertNumberOfCalls(t, "GetPodsWithDifferentImage", 0)
		action.AssertNumberOfCalls(t, "Reset", 0)
	})
}
