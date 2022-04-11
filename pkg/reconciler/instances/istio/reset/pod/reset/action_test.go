package reset

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/avast/retry-go"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/kubectl/pkg/util/podutils"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/pod"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/pod/mocks"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_DefaultPodsResetAction_Reset(t *testing.T) {
	simplePod := v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "name"}}
	simpleCustomObject := pod.CustomObject{Name: "name"}
	ctx := context.Background()
	defer ctx.Done()
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
		err := action.Reset(ctx, kubeClient, fixRetryOpts, v1.PodList{}, log, debug, fixWaitOpts)

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
		err := action.Reset(ctx, kubeClient, fixRetryOpts, v1.PodList{}, log, debug, fixWaitOpts)

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
		err := action.Reset(ctx, kubeClient, fixRetryOpts, v1.PodList{Items: []v1.Pod{simplePod, simplePod}}, log, debug, fixWaitOpts)

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

		handler.On("ExecuteAndWaitFor", mock.Anything, mock.AnythingOfType("pod.CustomObject")).Return(nil)
		matcher.On("GetHandlersMap", mock.Anything, mock.AnythingOfType("[]retry.Option"), mock.AnythingOfType("v1.PodList"), mock.AnythingOfType("*zap.SugaredLogger"), mock.AnythingOfType("bool"), mock.AnythingOfType("pod.WaitOptions")).
			Return(handlersMap)

		// when
		err := action.Reset(ctx, kubeClient, fixRetryOpts, v1.PodList{Items: []v1.Pod{simplePod}}, log, debug, fixWaitOpts)

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

		handler.On("ExecuteAndWaitFor", mock.Anything, mock.AnythingOfType("pod.CustomObject")).Return(nil)
		matcher.On("GetHandlersMap", mock.Anything, mock.AnythingOfType("[]retry.Option"), mock.AnythingOfType("v1.PodList"), mock.AnythingOfType("*zap.SugaredLogger"), mock.AnythingOfType("bool"), mock.AnythingOfType("pod.WaitOptions")).
			Return(handlersMap)

		// when
		err := action.Reset(ctx, kubeClient, fixRetryOpts, v1.PodList{Items: []v1.Pod{simplePod, simplePod}}, log, debug, fixWaitOpts)

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

		handler1.On("ExecuteAndWaitFor", mock.Anything, mock.AnythingOfType("pod.CustomObject")).Return(nil)
		handler2.On("ExecuteAndWaitFor", mock.Anything, mock.AnythingOfType("pod.CustomObject")).Return(nil)
		matcher.On("GetHandlersMap", mock.Anything, mock.AnythingOfType("[]retry.Option"), mock.AnythingOfType("v1.PodList"), mock.AnythingOfType("*zap.SugaredLogger"), mock.AnythingOfType("bool"), mock.AnythingOfType("pod.WaitOptions")).
			Return(handlersMap)

		// when
		err := action.Reset(ctx, kubeClient, fixRetryOpts, v1.PodList{Items: []v1.Pod{simplePod, simplePod}}, log, debug, fixWaitOpts)

		// then
		require.NoError(t, err)
		matcher.AssertNumberOfCalls(t, "GetHandlersMap", 1)
		handler1.AssertNumberOfCalls(t, "ExecuteAndWaitFor", 1)
		handler2.AssertNumberOfCalls(t, "ExecuteAndWaitFor", 1)
	})

	t.Run("given pod with not running state", func(t *testing.T) {
		// given
		podNs := "testNamespace"
		podName := "testPod"
		labels := map[string]string{"app": "test"}
		labelSelector := createLabelSelector(labels)
		deployment := createDeploymentWithLabelSelectors(podNs, labels)
		pendingPod := createPendingPodDeploymentOwnerRef(podName, podNs, deployment, labels)
		require.False(t, podutils.IsPodAvailable(&pendingPod, 1, metav1.Now()))

		t.Run("should annotate deployment if rollout is not successful", func(t *testing.T) {
			// given
			k8sclient := fake.NewSimpleClientset(&pendingPod, &deployment)
			action := NewDefaultPodsResetAction(pod.NewParentKindMatcher())
			pl := v1.PodList{
				Items: []v1.Pod{pendingPod},
			}
			fixWaitOpts.Interval = time.Millisecond * 10
			fixWaitOpts.Timeout = time.Millisecond * 100

			// when
			err := action.Reset(context.Background(), k8sclient, fixRetryOpts, pl, log, false, fixWaitOpts)
			require.Error(t, err)

			// then
			got, err := k8sclient.CoreV1().Pods(podNs).List(ctx, metav1.ListOptions{
				LabelSelector: labelSelector,
			})
			require.NoError(t, err)
			for _, p := range got.Items {
				v, ok := p.Annotations[pod.AnnotationResetWarningKey]
				require.True(t, ok)
				require.Equal(t, pod.AnnotationResetWarningRolloutTimeoutVal, v)
			}
		})
	})

	t.Run("given pod without owner ref, should annotate", func(t *testing.T) {
		// given
		podNs := "testNamespace"
		podName := "testPod"
		pendingPod := v1.Pod{
			TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: podNs,
			},
		}
		k8sclient := fake.NewSimpleClientset(&pendingPod)
		action := NewDefaultPodsResetAction(pod.NewParentKindMatcher())
		pl := v1.PodList{
			Items: []v1.Pod{pendingPod},
		}

		// when
		err := action.Reset(context.Background(), k8sclient, fixRetryOpts, pl, log, false, fixWaitOpts)
		require.NoError(t, err)

		// then
		got, err := k8sclient.CoreV1().Pods(podNs).Get(context.Background(), podName, metav1.GetOptions{})
		require.NoError(t, err)
		v, ok := got.Annotations[pod.AnnotationResetWarningKey]
		require.True(t, ok)
		require.Equal(t, pod.AnnotationResetWarningNoOwnerVal, v)
	})

	t.Run("given pods in not running state but they have been already annotated, no action should be performed", func(t *testing.T) {
		// given
		matcher := mocks.Matcher{}
		action := NewDefaultPodsResetAction(&matcher)
		simplePodAnnotated := simplePod
		if simplePodAnnotated.Annotations == nil {
			simplePodAnnotated.Annotations = make(map[string]string)
		}
		simplePodAnnotated.Annotations[pod.AnnotationResetWarningKey] = "test"
		pods := []v1.Pod{simplePodAnnotated, simplePod}

		handlersMap := map[pod.Handler][]pod.CustomObject{}

		matcher.On("GetHandlersMap", mock.Anything, mock.AnythingOfType("[]retry.Option"), mock.AnythingOfType("v1.PodList"), mock.AnythingOfType("*zap.SugaredLogger"), mock.AnythingOfType("bool"), mock.AnythingOfType("pod.WaitOptions")).
			Return(handlersMap)

		// when
		err := action.Reset(ctx, kubeClient, fixRetryOpts, v1.PodList{Items: pods}, log, debug, fixWaitOpts)

		// then
		require.NoError(t, err)
		shouldBeCalledWithoutAnnotatePod := func(t testing.TB) {
			t.Helper()
			matcher.AssertCalled(t, "GetHandlersMap", kubeClient, fixRetryOpts, v1.PodList{Items: []v1.Pod{simplePod}}, log, debug, fixWaitOpts)
		}
		shouldBeCalledWithoutAnnotatePod(t)
	})
}

func createLabelSelector(labels map[string]string) string {
	var labelSelector string
	for k, v := range labels {
		labelSelector = fmt.Sprintf("%s=%s", k, v)
	}
	return labelSelector
}

func createPendingPodDeploymentOwnerRef(podName string, namespace string, deployment appsv1.Deployment, labels map[string]string) v1.Pod {
	pendingPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{{
				Kind: deployment.Kind,
				Name: deployment.Name,
			}},
			Labels: labels,
		},
		Spec: v1.PodSpec{},
		Status: v1.PodStatus{
			Phase: "Pending",
		},
	}
	return pendingPod
}

func createDeploymentWithLabelSelectors(namespace string, labels map[string]string) appsv1.Deployment {
	deployment := appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind: "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deployment-image-backoff-pending",
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: v1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: labels}},
		},
	}
	return deployment
}
