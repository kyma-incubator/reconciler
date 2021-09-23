package proxy

import (
	"errors"
	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/config"
	datamocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/data/mocks"
	podresetmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/pod/reset/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
)

func Test_IstioProxyReset_Run(t *testing.T) {
	cfg := config.IstioProxyConfig{
		ImagePrefix:           "istio/proxyv2",
		ImageVersion:          "1.10.2",
		RetriesCount:          5,
		SleepAfterPodDeletion: 10,
		Log:                   log.NewOptionalLogger(true),
	}

	t.Run("should not return an error when no pods are present on the cluster", func(t *testing.T) {
		// given
		gatherer := datamocks.Gatherer{}
		gatherer.On("GetAllPods").Return(&v1.PodList{}, nil)
		gatherer.On("GetPodsWithDifferentImage", mock.AnythingOfType("v1.PodList"),
			mock.AnythingOfType("data.ExpectedImage")).Return(v1.PodList{})

		action := podresetmocks.ResetAction{}
		action.On("Reset", mock.AnythingOfType("v1.PodList")).Return(nil)
		istioProxyReset := NewIstioProxyReset(cfg, &gatherer, &action)

		// when
		err := istioProxyReset.Run()

		// then
		require.NoError(t, err)
		gatherer.AssertNumberOfCalls(t, "GetAllPods", 1)
		gatherer.AssertNumberOfCalls(t, "GetPodsWithDifferentImage", 1)
		action.AssertNumberOfCalls(t, "Reset", 1)
	})

	t.Run("should not return an error when pods are present on the cluster", func(t *testing.T) {
		// given
		gatherer := datamocks.Gatherer{}
		gatherer.On("GetAllPods").Return(&v1.PodList{Items: []v1.Pod{{}}}, nil)
		gatherer.On("GetPodsWithDifferentImage", mock.AnythingOfType("v1.PodList"),
			mock.AnythingOfType("data.ExpectedImage")).Return(v1.PodList{Items: []v1.Pod{{}}})

		action := podresetmocks.ResetAction{}
		action.On("Reset", mock.AnythingOfType("v1.PodList")).Return(nil)
		istioProxyReset := NewIstioProxyReset(cfg, &gatherer, &action)

		// when
		err := istioProxyReset.Run()

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
		gatherer.On("GetAllPods").Return(nil, expectedError)
		gatherer.On("GetPodsWithDifferentImage", mock.AnythingOfType("v1.PodList"),
			mock.AnythingOfType("data.ExpectedImage")).Return(v1.PodList{})

		action := podresetmocks.ResetAction{}
		action.On("Reset", mock.AnythingOfType("v1.PodList")).Return(nil)
		istioProxyReset := IstioProxyReset{cfg, &gatherer, &action}

		// when
		err := istioProxyReset.Run()

		// then
		require.ErrorIs(t, err, expectedError)
		gatherer.AssertNumberOfCalls(t, "GetAllPods", 1)
		gatherer.AssertNumberOfCalls(t, "GetPodsWithDifferentImage", 0)
		action.AssertNumberOfCalls(t, "Reset", 0)
	})
}
