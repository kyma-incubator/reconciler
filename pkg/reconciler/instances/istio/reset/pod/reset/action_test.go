package reset

import (
	"testing"
	"time"

	"github.com/avast/retry-go"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/pod"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/pod/mocks"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_DefaultPodsResetAction_Reset(t *testing.T) {
	simplePod := v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "name"}}
	simpleCustomObject := pod.CustomObject{Name: "name"}
	log := logger.NewLogger(true)
	debug := true
	fixRetryOpts := []retry.Option{
		retry.Delay(1 * time.Second),
		retry.Attempts(1),
		retry.DelayType(retry.FixedDelay),
	}
	fixWaitOpts := pod.WaitOptions{
		Timeout:  time.Duration(5) * time.Minute,
		Interval: time.Duration(5) * time.Second,
	}
	kubeClient := fake.NewSimpleClientset()

	t.Run("should not reset any pod from an empty list of pods when no handlers are available", func(t *testing.T) {
		// given
		matcher := mocks.Matcher{}
		action := NewDefaultPodsResetAction(&matcher)
		matcher.On("GetHandlersMap", mock.Anything, mock.AnythingOfType("[]retry.Option"), mock.AnythingOfType("v1.PodList"), mock.AnythingOfType("*zap.SugaredLogger"), mock.AnythingOfType("bool"), mock.AnythingOfType("pod.WaitOptions")).
			Return(nil)

		// when
		err := action.Reset(kubeClient, fixRetryOpts, v1.PodList{}, log, debug, fixWaitOpts)

		// then
		require.NoError(t, err)
		matcher.AssertNumberOfCalls(t, "GetHandlersMap", 1)
	})

	t.Run("should not reset any pod for an empty list of pods when a single handler is available", func(t *testing.T) {
		// given
		matcher := mocks.Matcher{}
		action := NewDefaultPodsResetAction(&matcher)
		handler := mocks.Handler{}
		handlersMap := map[pod.Handler][]pod.CustomObject{&handler: {}}
		matcher.On("GetHandlersMap", mock.Anything, mock.AnythingOfType("[]retry.Option"), mock.AnythingOfType("v1.PodList"), mock.AnythingOfType("*zap.SugaredLogger"), mock.AnythingOfType("bool"), mock.AnythingOfType("pod.WaitOptions")).
			Return(handlersMap)

		// when
		err := action.Reset(kubeClient, fixRetryOpts, v1.PodList{}, log, debug, fixWaitOpts)

		// then
		require.NoError(t, err)
		matcher.AssertNumberOfCalls(t, "GetHandlersMap", 1)
		handler.AssertNumberOfCalls(t, "ExecuteAndWaitFor", 0)
	})

	t.Run("should not reset any pod from list of pods when no single handler is available", func(t *testing.T) {
		// given
		matcher := mocks.Matcher{}
		action := NewDefaultPodsResetAction(&matcher)
		matcher.On("GetHandlersMap", mock.Anything, mock.AnythingOfType("[]retry.Option"), mock.AnythingOfType("v1.PodList"), mock.AnythingOfType("*zap.SugaredLogger"), mock.AnythingOfType("bool"), mock.AnythingOfType("pod.WaitOptions")).
			Return(nil)

		// when
		err := action.Reset(kubeClient, fixRetryOpts, v1.PodList{Items: []v1.Pod{simplePod, simplePod}}, log, debug, fixWaitOpts)

		// then
		require.NoError(t, err)
		matcher.AssertNumberOfCalls(t, "GetHandlersMap", 1)
	})

	t.Run("should reset the single pod from the list of pods when a single handler is available", func(t *testing.T) {
		// given
		matcher := mocks.Matcher{}
		action := NewDefaultPodsResetAction(&matcher)
		handler := mocks.Handler{}
		handlersMap := map[pod.Handler][]pod.CustomObject{&handler: {simpleCustomObject}}

		handler.On("ExecuteAndWaitFor", mock.AnythingOfType("pod.CustomObject")).Return(nil)
		matcher.On("GetHandlersMap", mock.Anything, mock.AnythingOfType("[]retry.Option"), mock.AnythingOfType("v1.PodList"), mock.AnythingOfType("*zap.SugaredLogger"), mock.AnythingOfType("bool"), mock.AnythingOfType("pod.WaitOptions")).
			Return(handlersMap)

		// when
		err := action.Reset(kubeClient, fixRetryOpts, v1.PodList{Items: []v1.Pod{simplePod}}, log, debug, fixWaitOpts)

		// then
		require.NoError(t, err)
		matcher.AssertNumberOfCalls(t, "GetHandlersMap", 1)
		handler.AssertNumberOfCalls(t, "ExecuteAndWaitFor", 1)
	})

	t.Run("should reset two pods from the list of pods when a singler handler is available", func(t *testing.T) {
		// given
		matcher := mocks.Matcher{}
		action := NewDefaultPodsResetAction(&matcher)
		handler := mocks.Handler{}
		handlersMap := map[pod.Handler][]pod.CustomObject{&handler: {simpleCustomObject, simpleCustomObject}}

		handler.On("ExecuteAndWaitFor", mock.AnythingOfType("pod.CustomObject")).Return(nil)
		matcher.On("GetHandlersMap", mock.Anything, mock.AnythingOfType("[]retry.Option"), mock.AnythingOfType("v1.PodList"), mock.AnythingOfType("*zap.SugaredLogger"), mock.AnythingOfType("bool"), mock.AnythingOfType("pod.WaitOptions")).
			Return(handlersMap)

		// when
		err := action.Reset(kubeClient, fixRetryOpts, v1.PodList{Items: []v1.Pod{simplePod, simplePod}}, log, debug, fixWaitOpts)

		// then
		require.NoError(t, err)
		matcher.AssertNumberOfCalls(t, "GetHandlersMap", 1)
		handler.AssertNumberOfCalls(t, "ExecuteAndWaitFor", 2)
	})

	t.Run("should reset two pods from the list of pods when two handlers are available", func(t *testing.T) {
		// given
		matcher := mocks.Matcher{}
		action := NewDefaultPodsResetAction(&matcher)
		handler1 := mocks.Handler{}
		handler2 := mocks.Handler{}
		handlersMap := map[pod.Handler][]pod.CustomObject{&handler1: {simpleCustomObject}, &handler2: {simpleCustomObject}}

		handler1.On("ExecuteAndWaitFor", mock.AnythingOfType("pod.CustomObject")).Return(nil)
		handler2.On("ExecuteAndWaitFor", mock.AnythingOfType("pod.CustomObject")).Return(nil)
		matcher.On("GetHandlersMap", mock.Anything, mock.AnythingOfType("[]retry.Option"), mock.AnythingOfType("v1.PodList"), mock.AnythingOfType("*zap.SugaredLogger"), mock.AnythingOfType("bool"), mock.AnythingOfType("pod.WaitOptions")).
			Return(handlersMap)

		// when
		err := action.Reset(kubeClient, fixRetryOpts, v1.PodList{Items: []v1.Pod{simplePod, simplePod}}, log, debug, fixWaitOpts)

		// then
		require.NoError(t, err)
		matcher.AssertNumberOfCalls(t, "GetHandlersMap", 1)
		handler1.AssertNumberOfCalls(t, "ExecuteAndWaitFor", 1)
		handler2.AssertNumberOfCalls(t, "ExecuteAndWaitFor", 1)
	})
}
