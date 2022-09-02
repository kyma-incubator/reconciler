package label_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/avast/retry-go"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/consts"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/data"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/label"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/pod/mocks"
	"go.uber.org/zap"

	gathererMocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/data/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	log "github.com/kyma-incubator/reconciler/pkg/logger"
)

const (
	testNs                         string = "test-ns"
	testNsSidecarInjectionDisabled string = "test-ns-disabled"
	testPodName                    string = "test-pod"
)

func Test_Run(t *testing.T) {
	rOpts := []retry.Option{
		retry.Delay(0),
		retry.Attempts(uint(1)),
		retry.DelayType(retry.FixedDelay),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watcherStarted := make(chan struct{})
	client := fake.NewSimpleClientset()

	client.PrependWatchReactor("*", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
		gvr := action.GetResource()
		ns := action.GetNamespace()
		watch, err := client.Tracker().Watch(gvr, ns)
		if err != nil {
			return false, nil, err
		}
		close(watcherStarted)
		return true, watch, nil
	})

	pods := make(chan *v1.Pod, 1)
	informers := informers.NewSharedInformerFactory(client, 0)
	podInformer := informers.Core().V1().Pods().Informer()
	podInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(_, newObj interface{}) {
			pod := newObj.(*v1.Pod)
			pods <- pod
		},
	})
	informers.Start(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), podInformer.HasSynced)

	<-watcherStarted

	_, err := client.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNs}}, metav1.CreateOptions{})
	require.NoError(t, err)

	_, err = client.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNsSidecarInjectionDisabled, Labels: map[string]string{"reconciler/namespace-istio-injection": "disabled"}}}, metav1.CreateOptions{})
	require.NoError(t, err)

	gatherer := data.NewDefaultGatherer()

	matcher := mocks.Matcher{}
	labelAction := label.NewDefaultPodsLabelAction(gatherer, &matcher)
	log := log.NewLogger(false)
	defer ctx.Done()

	t.Run("should label pods in namespace with reconciler/namespace-istio-injection=disabled when default istio injection is disabled", func(t *testing.T) {
		// Given
		simplePod := v1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: testNsSidecarInjectionDisabled, Name: testPodName}}
		defer func() {
			err = client.CoreV1().Pods(testNsSidecarInjectionDisabled).Delete(context.TODO(), simplePod.Name, metav1.DeleteOptions{})
			require.NoError(t, err)
		}()
		_, err := client.CoreV1().Pods(testNsSidecarInjectionDisabled).Create(context.TODO(), &simplePod, metav1.CreateOptions{})
		require.NoError(t, err)

		// When
		err = labelAction.Run(ctx, log, client, rOpts, false)

		//Then
		require.NoError(t, err)
		select {
		case pod := <-pods:
			require.Equal(t, pod.Labels[consts.KymaWarning], consts.NotInIstioMeshLabel)
		case <-time.After(time.Second):
			require.Fail(t, "Pod wasn't labeled")
		}

	})

	t.Run("should label pods which have annotation sidecar.istio.io/inject=false if istio injection is enabled by default", func(t *testing.T) {
		// Given
		annotatedPod := v1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: testNs, Name: testPodName, Annotations: map[string]string{"sidecar.istio.io/inject": "false"}}}
		_, err := client.CoreV1().Pods(testNs).Create(context.TODO(), &annotatedPod, metav1.CreateOptions{})
		require.NoError(t, err)
		defer func() {
			err = client.CoreV1().Pods(testNs).Delete(context.TODO(), annotatedPod.Name, metav1.DeleteOptions{})
			require.NoError(t, err)
		}()

		// When
		err = labelAction.Run(ctx, log, client, rOpts, true)

		//Then
		require.NoError(t, err)
		select {
		case pod := <-pods:
			require.Equal(t, pod.Labels[consts.KymaWarning], consts.NotInIstioMeshLabel)
		case <-time.After(time.Second):
			require.Fail(t, "Pod wasn't labeled")
		}
	})

	t.Run("should label pods with no namespace label and pod annotation if injection is disabled by default", func(t *testing.T) {
		// Given
		annotatedPod := v1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: testNs, Name: testPodName}}

		_, err := client.CoreV1().Pods(testNs).Create(context.TODO(), &annotatedPod, metav1.CreateOptions{})
		require.NoError(t, err)
		defer func() {
			err = client.CoreV1().Pods(testNs).Delete(context.TODO(), annotatedPod.Name, metav1.DeleteOptions{})
			require.NoError(t, err)
		}()

		// When
		err = labelAction.Run(ctx, log, client, rOpts, false)

		//Then
		require.NoError(t, err)
		select {
		case pod := <-pods:
			require.Equal(t, pod.Labels[consts.KymaWarning], consts.NotInIstioMeshLabel)
		case <-time.After(time.Second):
			require.Fail(t, "Pod wasn't labeled")
		}
	})

	t.Run("should not label pods with no annotation in namespace with no label when sidecar injection by default is enabled", func(t *testing.T) {
		// Given
		simplePod := v1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: testNs, Name: testPodName}}
		defer func() {
			err = client.CoreV1().Pods(testNs).Delete(context.TODO(), simplePod.Name, metav1.DeleteOptions{})
			require.NoError(t, err)
		}()
		_, err := client.CoreV1().Pods(testNs).Create(context.TODO(), &simplePod, metav1.CreateOptions{})
		require.NoError(t, err)

		// When
		err = labelAction.Run(ctx, log, client, rOpts, true)

		//Then
		require.NoError(t, err)
		require.NoError(t, err)
		select {
		case <-pods:
			require.Fail(t, "Pod was labeled but shouldn't")
		case <-time.After(time.Second):
			require.Empty(t, pods)
		}
	})

	t.Run("should not label pods in kyma-system, kube-system and kyma-integration", func(t *testing.T) {
		// Given
		_, err = client.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: consts.KymaSystem}}, metav1.CreateOptions{})
		require.NoError(t, err)
		_, err = client.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: consts.KubeSystem}}, metav1.CreateOptions{})
		require.NoError(t, err)
		_, err = client.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: consts.KymaIntegration}}, metav1.CreateOptions{})
		require.NoError(t, err)

		kymaSystemPod := v1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: consts.KymaSystem, Name: testPodName, Annotations: map[string]string{"sidecar.istio.io/inject": "false"}}}
		kubeSystemPod := v1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: consts.KubeSystem, Name: testPodName, Annotations: map[string]string{"sidecar.istio.io/inject": "false"}}}
		kymaIntegrationPod := v1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: consts.KymaIntegration, Name: testPodName, Annotations: map[string]string{"sidecar.istio.io/inject": "false"}}}
		defer func() {
			err = client.CoreV1().Namespaces().Delete(context.TODO(),consts.KubeSystem, metav1.DeleteOptions{})
			require.NoError(t, err)
			err = client.CoreV1().Namespaces().Delete(context.TODO(),consts.KymaSystem, metav1.DeleteOptions{})
			require.NoError(t, err)
			err = client.CoreV1().Namespaces().Delete(context.TODO(),consts.KymaIntegration, metav1.DeleteOptions{})
			require.NoError(t, err)
		}()

		_, err := client.CoreV1().Pods(consts.KymaSystem).Create(context.TODO(), &kymaSystemPod, metav1.CreateOptions{})
		require.NoError(t, err)

		_, err = client.CoreV1().Pods(consts.KubeSystem).Create(context.TODO(), &kubeSystemPod, metav1.CreateOptions{})
		require.NoError(t, err)

		_, err = client.CoreV1().Pods(consts.KymaIntegration).Create(context.TODO(), &kymaIntegrationPod, metav1.CreateOptions{})
		require.NoError(t, err)

		// When
		err = labelAction.Run(ctx, log, client, rOpts, true)

		// Then
		require.NoError(t, err)
		select {
		case <-pods:
			require.Fail(t, "Pod was labeled but shouldn't")
		case <-time.After(time.Second):
			require.Empty(t, pods)
		}
	})
}

func Test_ErrorHandlingAction(t *testing.T) {
	t.Run("should return an error if gatherer.GetPodsOutOfIstioMesh returns an error", func(t *testing.T) {
		// Given
		gatherer := gathererMocks.NewGatherer(t)
		gatherer.On("GetPodsOutOfIstioMesh", mock.Anything, mock.Anything, mock.Anything).Return(v1.PodList{}, errors.New("some error"))
		labelAction := label.NewDefaultPodsLabelAction(gatherer, &mocks.Matcher{})

		// When
		err := labelAction.Run(context.TODO(), &zap.SugaredLogger{}, fake.NewSimpleClientset(), []retry.Option{}, false)

		// Then
		require.Error(t, err)
	})
}
