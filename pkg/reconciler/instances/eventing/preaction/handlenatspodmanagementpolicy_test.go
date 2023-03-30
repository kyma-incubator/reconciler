package preaction

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/log"
	k8s "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/progress"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/kustomize/kyaml/yaml"

	chartmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/chart/mocks"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/require"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const (
	natsComponentName = "eventing-nats"
	volumeClaimName   = natsComponentName + "-js-pvc"
	pvcName           = volumeClaimName + "-" + statefulSetName + "-"
)

func Test_getNATSChartPodManagementPolicy(t *testing.T) {
	testCases := []struct {
		name                     string
		givenWithPolicy          bool
		givenPodManagementPolicy string
		wantPodManagementPolicy  string
	}{
		{
			name:                     "Should return Parallel pod management policy",
			givenWithPolicy:          true,
			givenPodManagementPolicy: string(appsv1.ParallelPodManagement),
			wantPodManagementPolicy:  string(appsv1.ParallelPodManagement),
		},
		{
			name:                     "Should return OrderedReady pod management policy",
			givenWithPolicy:          true,
			givenPodManagementPolicy: string(appsv1.OrderedReadyPodManagement),
			wantPodManagementPolicy:  string(appsv1.OrderedReadyPodManagement),
		},
		{
			name:                    "Should return empty pod management policy",
			givenWithPolicy:         false,
			wantPodManagementPolicy: "",
		},
	}

	// test setup function
	setup := func(t *testing.T, withPolicy bool, policy string) (handleNATSPodManagementPolicy, *service.ActionContext) {
		action := handleNATSPodManagementPolicy{
			kubeClientProvider: func(context *service.ActionContext, logger *zap.SugaredLogger) (k8s.Client, error) {
				return nil, nil
			},
		}

		chartProvider := &chartmocks.Provider{}
		chartValuesYAML := getJetStreamValuesYAML(withPolicy, policy)
		chartValues, err := unmarshalTestValues(chartValuesYAML)
		require.NoError(t, err)

		chartProvider.On("Configuration", mock.Anything).Return(chartValues, nil)

		actionContext := &service.ActionContext{
			Context:       context.TODO(),
			Logger:        logger.NewLogger(false),
			Task:          &reconciler.Task{Version: "test"},
			ChartProvider: chartProvider,
		}
		return action, actionContext
	}

	// test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			action, actionContext := setup(t, tc.givenWithPolicy, tc.givenPodManagementPolicy)

			// when
			policy, err := action.getNATSChartPodManagementPolicy(actionContext)

			// then
			require.NoError(t, err)
			require.Equal(t, tc.wantPodManagementPolicy, policy)
		})
	}
}

func TestHandleNATSPodManagementPolicy(t *testing.T) {
	testCases := []struct {
		name                     string
		givenStatefulSet         *appsv1.StatefulSet
		givenWithPolicy          bool
		givenPodManagementPolicy string
		wantPVCToExists          bool
		wantStatefulSetDeletion  bool
		wantPodManagementPolicy  appsv1.PodManagementPolicyType
	}{
		{
			name:                    "Should do nothing if Nats StatefulSet is not found",
			givenWithPolicy:         false,
			givenStatefulSet:        nil,
			wantStatefulSetDeletion: false,
			wantPVCToExists:         false,
		},
		{
			name:                    "Should not delete Nats StatefulSet if pod management policy is not set in helm chart",
			givenWithPolicy:         false,
			givenStatefulSet:        newStatefulSet(withPodManagementPolicy(appsv1.OrderedReadyPodManagement)),
			wantStatefulSetDeletion: false,
			wantPVCToExists:         true,
			wantPodManagementPolicy: appsv1.OrderedReadyPodManagement,
		},
		{
			name:                     "Should delete Nats StatefulSet if pod management policy is not Parallel",
			givenPodManagementPolicy: string(appsv1.ParallelPodManagement),
			givenWithPolicy:          true,
			givenStatefulSet:         newStatefulSet(withPodManagementPolicy(appsv1.OrderedReadyPodManagement)),
			wantStatefulSetDeletion:  true,
			wantPVCToExists:          true,
			wantPodManagementPolicy:  appsv1.ParallelPodManagement,
		},
		{
			name:                     "Should not delete Nats StatefulSet if pod management policy is Parallel",
			givenPodManagementPolicy: string(appsv1.ParallelPodManagement),
			givenWithPolicy:          true,
			givenStatefulSet:         newStatefulSet(withPodManagementPolicy(appsv1.ParallelPodManagement)),
			wantStatefulSetDeletion:  false,
			wantPVCToExists:          true,
			wantPodManagementPolicy:  appsv1.ParallelPodManagement,
		},
	}

	// test setup function
	setup := func(t *testing.T, withPolicy bool, policy string) (kubernetes.Interface, handleNATSPodManagementPolicy, *service.ActionContext) {
		k8sClient := fake.NewSimpleClientset()
		mockClient := mocks.Client{}
		mockClient.On("Clientset").Return(k8sClient, nil)
		action := handleNATSPodManagementPolicy{
			kubeClientProvider: func(context *service.ActionContext, logger *zap.SugaredLogger) (k8s.Client, error) {
				return &mockClient, nil
			},
		}

		chartProvider := &chartmocks.Provider{}
		chartValuesYAML := getJetStreamValuesYAML(withPolicy, policy)
		chartValues, err := unmarshalTestValues(chartValuesYAML)
		require.NoError(t, err)

		chartProvider.On("Configuration", mock.Anything).Return(chartValues, nil)

		actionContext := &service.ActionContext{
			KubeClient:    &mockClient,
			Context:       context.TODO(),
			Logger:        logger.NewLogger(false),
			Task:          &reconciler.Task{Version: "test"},
			ChartProvider: chartProvider,
		}
		return k8sClient, action, actionContext
	}

	// test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			var err error
			replicaCount := 0
			k8sClient, action, actionContext := setup(t, tc.givenWithPolicy, tc.givenPodManagementPolicy)

			if tc.givenStatefulSet != nil {
				// create NATS statefulset
				_, err = createStatefulSet(actionContext.Context, k8sClient, tc.givenStatefulSet)
				require.NoError(t, err)

				// create PVCs according to replicas number
				replicaCount = int(*tc.givenStatefulSet.Spec.Replicas)
				if replicaCount > 0 {
					for i := 0; i < replicaCount; i++ {
						err := createPVC(actionContext.Context, k8sClient, pvcName+fmt.Sprint(i))
						require.NoError(t, err)
					}
					list, err := k8sClient.CoreV1().PersistentVolumeClaims(namespace).List(actionContext.Context, metav1.ListOptions{})
					require.NoError(t, err)
					assert.Equal(t, replicaCount, len(list.Items))
				}
			}

			// when
			err = action.Execute(actionContext, log.ContextLogger(actionContext, log.WithAction(actionName)))
			require.NoError(t, err)

			// then
			if tc.wantPVCToExists {
				// check if all the PVC instances were deleted
				list, err := k8sClient.CoreV1().PersistentVolumeClaims(namespace).List(actionContext.Context, metav1.ListOptions{})
				require.NoError(t, err)
				assert.Equal(t, replicaCount, len(list.Items))
			}

			gotStatefulSet, err := k8sClient.AppsV1().StatefulSets(namespace).Get(actionContext.Context, statefulSetName, metav1.GetOptions{})
			if !k8serrors.IsNotFound(err) {
				require.NoError(t, err)
			}

			if tc.wantStatefulSetDeletion {
				// statefulset should not exists
				require.Nil(t, gotStatefulSet)
			} else if tc.givenStatefulSet != nil {
				// if statefulset exists then check its pod management policy
				require.NotNil(t, gotStatefulSet)
				require.Equal(t, tc.wantPodManagementPolicy, gotStatefulSet.Spec.PodManagementPolicy)
			}
		})
	}
}

// Test_deleteNATSStatefulSet_failIfPodIsNotTerminated makes sure that deleteNATSStatefulSet returns an err if the Pod
// is never terminated.
func Test_deleteNATSStatefulSet_failIfPodIsNotTerminated(t *testing.T) {
	// Assert.
	k8sClient := fake.NewSimpleClientset()
	mockClient := mocks.Client{}
	mockClient.On("Clientset").Return(k8sClient, nil)

	chartProvider := &chartmocks.Provider{}
	chartValuesYAML := getJetStreamValuesYAML(true, string(appsv1.ParallelPodManagement))
	chartValues, err := unmarshalTestValues(chartValuesYAML)
	require.NoError(t, err)

	chartProvider.On("Configuration", mock.Anything).Return(chartValues, nil)

	actionContext := &service.ActionContext{
		KubeClient:    &mockClient,
		Context:       context.TODO(),
		Logger:        logger.NewLogger(false),
		Task:          &reconciler.Task{Version: "test"},
		ChartProvider: chartProvider,
	}

	// Create NATS StatefulSet.
	sts := newStatefulSet(withPodManagementPolicy(appsv1.OrderedReadyPodManagement))
	_, err = createStatefulSet(actionContext.Context, k8sClient, sts)
	require.NoError(t, err)

	// Create Pod. This Pod is not associated with the StatefulSet, so it will not be terminated if the sts gets removed.
	pod := newPod(withLabels(map[string]string{"app.kubernetes.io/name": "nats"}))
	err = createPod(actionContext.Context, k8sClient, pod)
	require.NoError(t, err)

	// Set tracker.
	//todo logger
	lgr := logger.NewTestLogger(t)
	tracker, err := progress.NewProgressTracker(k8sClient, lgr,
		progress.Config{
			Interval: progressTrackerInterval,
			Timeout:  10 * time.Second,
		},
	)
	require.NoError(t, err)

	// Act.
	err = deleteNATSStatefulSet(actionContext, k8sClient, tracker, lgr)

	// Asses.
	require.Error(t, err)
	expectedErr := "progress tracker reached timeout (10 secs): stop checking progress of resource transition to state 'terminated'"
	require.Equal(t, expectedErr, err.Error())
}

type podOpt func(pod *corev1.Pod)

func newPod(opts ...podOpt) *corev1.Pod {
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      uuid.NewString(),
			Namespace: namespace,
		},
	}

	for _, opt := range opts {
		opt(pod)
	}

	return pod
}

func withLabels(labels map[string]string) podOpt {
	return func(pod *corev1.Pod) {
		pod.SetLabels(labels)
	}
}

type statefulSetOpt func(set *appsv1.StatefulSet)

func newStatefulSet(opts ...statefulSetOpt) *appsv1.StatefulSet {
	replicas := int32(1)
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      statefulSetName,
			Namespace: namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
		},
	}

	for _, opt := range opts {
		opt(statefulSet)
	}

	return statefulSet
}

func withPodManagementPolicy(policy appsv1.PodManagementPolicyType) statefulSetOpt {
	return func(statefulSet *appsv1.StatefulSet) {
		statefulSet.Spec.PodManagementPolicy = policy
	}
}

func getJetStreamValuesYAML(withPolicy bool, policy string) string {
	if !withPolicy {
		return `
    global:
      jetstream:
        enabled: true`
	}

	return fmt.Sprintf(`
    global:
      jetstream:
        podManagementPolicy: %s`,
		policy,
	)
}

func unmarshalTestValues(yamlValues string) (map[string]interface{}, error) {
	var values map[string]interface{}
	err := yaml.Unmarshal([]byte(yamlValues), &values)
	if err != nil {
		return nil, err
	}
	return values, nil
}

func createStatefulSet(ctx context.Context, client kubernetes.Interface, statefulSet *appsv1.StatefulSet) (*appsv1.StatefulSet, error) {
	return client.AppsV1().StatefulSets(statefulSet.Namespace).Create(ctx, statefulSet, metav1.CreateOptions{})
}

func createPod(ctx context.Context, client kubernetes.Interface, pod *corev1.Pod) error {
	if _, err := client.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		return err
	}
	return nil
}
func createPVC(ctx context.Context, client kubernetes.Interface, name string) error {
	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	if _, err := client.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{}); err != nil {
		return err
	}
	return nil
}
