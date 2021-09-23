package reset

import (
	"github.com/avast/retry-go"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"k8s.io/client-go/kubernetes/fake"
	"sync"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/pod"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/pod/mocks"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_DefaultPodsResetAction_Reset(t *testing.T) {
	simplePod := v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "name"}}
	simpleCustomObject := pod.CustomObject{Name: "name"}
	log := logger.NewOptionalLogger(true)
	debug := true
	fixRetryOpts := []retry.Option{
		retry.Delay(1 * time.Second),
		retry.Attempts(1),
		retry.DelayType(retry.FixedDelay),
	}
	kubeClient := fake.NewSimpleClientset()

	t.Run("should not execute any handler for an empty list of pods when no handlers are available", func(t *testing.T) {
		// given
		matcher := mocks.Matcher{}
		action := NewDefaultPodsResetAction(&matcher)
		matcher.On("GetHandlersMap", mock.Anything, mock.AnythingOfType("[]retry.Option"), mock.AnythingOfType("v1.PodList"), mock.AnythingOfType("*zap.SugaredLogger"), mock.AnythingOfType("bool")).Return(nil)

		// when
		action.Reset(kubeClient, fixRetryOpts, v1.PodList{}, log, debug)

		// then
		matcher.AssertNumberOfCalls(t, "GetHandlersMap", 1)
	})

	t.Run("should not execute any handler for an empty list of pods when a single handler is available", func(t *testing.T) {
		// given
		matcher := mocks.Matcher{}
		action := NewDefaultPodsResetAction(&matcher)
		handler := mocks.Handler{}
		handlersMap := map[pod.Handler][]pod.CustomObject{&handler: {}}
		matcher.On("GetHandlersMap", mock.Anything, mock.AnythingOfType("[]retry.Option"), mock.AnythingOfType("v1.PodList"), mock.AnythingOfType("*zap.SugaredLogger"), mock.AnythingOfType("bool")).Return(handlersMap)

		// when
		action.Reset(kubeClient, fixRetryOpts, v1.PodList{}, log, debug)

		// then
		matcher.AssertNumberOfCalls(t, "GetHandlersMap", 1)
		handler.AssertNumberOfCalls(t, "Execute", 0)
	})

	t.Run("should execute the handler for a single pod when a single handler is available", func(t *testing.T) {
		// given
		matcher := mocks.Matcher{}
		action := NewDefaultPodsResetAction(&matcher)
		handler := mocks.Handler{}
		handlersMap := map[pod.Handler][]pod.CustomObject{&handler: {simpleCustomObject}}

		handler.On("Execute", mock.AnythingOfType("pod.CustomObject"), &action.wg).Return(nil).Run(func(args mock.Arguments) {
			wg := args.Get(1).(*sync.WaitGroup)
			// wg.Done() must be called manually during execute
			wg.Done()
		})
		matcher.On("GetHandlersMap", mock.Anything, mock.AnythingOfType("[]retry.Option"), mock.AnythingOfType("v1.PodList"), mock.AnythingOfType("*zap.SugaredLogger"), mock.AnythingOfType("bool")).Return(handlersMap)

		// when
		action.Reset(kubeClient, fixRetryOpts, v1.PodList{Items: []v1.Pod{simplePod}}, log, debug)

		// then
		matcher.AssertNumberOfCalls(t, "GetHandlersMap", 1)
		handler.AssertNumberOfCalls(t, "Execute", 1)
	})

	t.Run("should execute the handler twice for two pods", func(t *testing.T) {
		// given
		matcher := mocks.Matcher{}
		action := NewDefaultPodsResetAction(&matcher)
		handler := mocks.Handler{}
		handlersMap := map[pod.Handler][]pod.CustomObject{&handler: {simpleCustomObject, simpleCustomObject}}

		handler.On("Execute", mock.AnythingOfType("pod.CustomObject"), &action.wg).Return(nil).Run(func(args mock.Arguments) {
			wg := args.Get(1).(*sync.WaitGroup)
			// wg.Done() must be called manually during execute
			wg.Done()
		})
		matcher.On("GetHandlersMap", mock.Anything, mock.AnythingOfType("[]retry.Option"), mock.AnythingOfType("v1.PodList"), mock.AnythingOfType("*zap.SugaredLogger"), mock.AnythingOfType("bool")).Return(handlersMap)

		// when
		action.Reset(kubeClient, fixRetryOpts, v1.PodList{Items: []v1.Pod{simplePod, simplePod}}, log, debug)

		// then
		matcher.AssertNumberOfCalls(t, "GetHandlersMap", 1)
		handler.AssertNumberOfCalls(t, "Execute", 2)
	})

	t.Run("should execute two handlers for two pods", func(t *testing.T) {
		// given
		matcher := mocks.Matcher{}
		action := NewDefaultPodsResetAction(&matcher)
		handler1 := mocks.Handler{}
		handler2 := mocks.Handler{}
		handlersMap := map[pod.Handler][]pod.CustomObject{&handler1: {simpleCustomObject}, &handler2: {simpleCustomObject}}

		handler1.On("Execute", mock.AnythingOfType("pod.CustomObject"), &action.wg).Return(nil).Run(func(args mock.Arguments) {
			wg := args.Get(1).(*sync.WaitGroup)
			// wg.Done() must be called manually during execute
			wg.Done()
		})
		handler2.On("Execute", mock.AnythingOfType("pod.CustomObject"), &action.wg).Return(nil).Run(func(args mock.Arguments) {
			wg := args.Get(1).(*sync.WaitGroup)
			// wg.Done() must be called manually during execute
			wg.Done()
		})
		matcher.On("GetHandlersMap", mock.Anything, mock.AnythingOfType("[]retry.Option"), mock.AnythingOfType("v1.PodList"), mock.AnythingOfType("*zap.SugaredLogger"), mock.AnythingOfType("bool")).Return(handlersMap)

		// when
		action.Reset(kubeClient, fixRetryOpts, v1.PodList{Items: []v1.Pod{simplePod, simplePod}}, log, debug)

		// then
		matcher.AssertNumberOfCalls(t, "GetHandlersMap", 1)
		handler1.AssertNumberOfCalls(t, "Execute", 1)
		handler2.AssertNumberOfCalls(t, "Execute", 1)
	})
}
