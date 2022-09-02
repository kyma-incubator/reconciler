package label

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/avast/retry-go"
	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/config"
	gathererMocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/data/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/pod/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

const (
	testNs                         string = "test-ns"
	testNsSidecarInjectionDisabled string = "test-ns-disabled"
	testPodName                    string = "test-pod"
)

func Test_Label_With_Warning(t *testing.T) {
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
		UpdateFunc: func(obj interface{}, newObj interface{}) {
			pod := newObj.(*v1.Pod)
			pods <- pod
		},
	})
	informers.Start(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), podInformer.HasSynced)

	<-watcherStarted

	_, err := client.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNs}}, metav1.CreateOptions{})
	require.NoError(t, err)

	log := log.NewLogger(false)
	defer ctx.Done()

	t.Run("should label with warning and return no error", func(t *testing.T) {
		// Given
		simplePod := v1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: testNs, Name: testPodName}}

		_, err := client.CoreV1().Pods(testNs).Create(context.TODO(), &simplePod, metav1.CreateOptions{})
		require.NoError(t, err)
		podList := v1.PodList{Items: []v1.Pod{simplePod}}

		// When
		err = labelWithWarning(ctx, client, wait.Backoff{Steps: 1}, podList, log)

		// Then
		require.NoError(t, err)
		select {
		case pod := <-pods:
			require.Equal(t, pod.Labels[config.KymaWarning], config.NotInIstioMeshLabel)
		case <-time.After(time.Second):
			require.Fail(t, "Pod wasn't labeled")
		}
	})

	t.Run("should return an error if patch returns an error", func(t *testing.T) {
		// Given
		simplePod := v1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: testNs, Name: "non existing pod"}}

		podList := v1.PodList{Items: []v1.Pod{simplePod}}

		// When
		err := labelWithWarning(ctx, client, wait.Backoff{Steps: 1}, podList, log)
		// Error: pod not found

		// Then
		require.Error(t, err)
	})
}

func Test_RunReturnErrorOnlabelWithWarningError(t *testing.T) {
	// Given
	gatherer := gathererMocks.NewGatherer(t)
	gatherer.On("GetPodsOutOfIstioMesh", mock.Anything, mock.Anything, mock.Anything).Return(v1.PodList{Items: []v1.Pod{{}}}, nil)
	labelAction := NewDefaultPodsLabelAction(gatherer, &mocks.Matcher{})
	labelWithWarning = func(context context.Context, kubeClient kubernetes.Interface, retryOpts wait.Backoff, podsList v1.PodList, log *zap.SugaredLogger) error {
		return errors.New("some error")
	}

	// When
	err := labelAction.Run(context.TODO(), &zap.SugaredLogger{}, fake.NewSimpleClientset(), []retry.Option{}, false)
	// Then
	require.Error(t, err)
}
