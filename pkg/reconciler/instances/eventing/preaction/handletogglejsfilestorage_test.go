package preaction

import (
	"context"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/log"
	k8s "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	"testing"

	chartmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/chart/mocks"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/require"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const pvcName = volumeClaimName + "-" + statefulSetName + "-"

func TestToggleJsFileStoragePreAction(t *testing.T) {
	const memoryStorageType = "memory"

	defaultStatefulSet := newStatefulSet()
	testCases := []struct {
		name             string
		givenStatefulSet *appsv1.StatefulSet
		givenJsEnabled   bool
		givenStorageType string
		givenPVC         bool
		wantStatefulSet  *appsv1.StatefulSet
		wantPVCDeletion  bool
	}{
		{
			name:             "Should do nothing if Nats StatefulSet is not found with JS disabled",
			givenJsEnabled:   false,
			givenStorageType: fileStorageType,
			givenStatefulSet: nil,
			givenPVC:         false,
			wantStatefulSet:  nil,
		},
		{
			name:             "Should do nothing if Nats StatefulSet is not found with JS enabled and file storage",
			givenJsEnabled:   false,
			givenStorageType: fileStorageType,
			givenStatefulSet: nil,
			givenPVC:         false,
			wantStatefulSet:  nil,
		},
		{
			name:             "Should do nothing if Nats StatefulSet is not found with JS enabled and memory storage",
			givenJsEnabled:   true,
			givenStorageType: memoryStorageType,
			givenStatefulSet: nil,
			givenPVC:         false,
			wantStatefulSet:  nil,
		},
		{
			name:             "Should do nothing if Nats Jetstream is disabled and there is no PVC",
			givenStatefulSet: defaultStatefulSet,
			givenJsEnabled:   false,
			givenPVC:         false,
			wantStatefulSet:  defaultStatefulSet,
		},
		{
			name:             "Should delete the StatefulSet if Nats Jetstream is disabled, but there is a PVC",
			givenStatefulSet: newStatefulSet(withVolumeClaimTemplates()),
			givenJsEnabled:   false,
			givenPVC:         true,
			wantStatefulSet:  nil,
		},
		{
			name:             "Should delete the StatefulSet if Nats Jetstream is enabled with file storage type and there is no PVC",
			givenStatefulSet: defaultStatefulSet,
			givenJsEnabled:   true,
			givenStorageType: fileStorageType,
			givenPVC:         false,
			wantStatefulSet:  nil,
		},
		{
			name:             "Should do nothing if Nats Jetstream is enabled with file storage type and there is a PVC",
			givenStatefulSet: defaultStatefulSet,
			givenJsEnabled:   true,
			givenStorageType: fileStorageType,
			givenPVC:         false,
			wantStatefulSet:  nil,
		},
		{
			name: "Should delete the StatefulSet and the PVC if Nats Jetstream is disabled and there is a PVC left",
			givenStatefulSet: newStatefulSet(
				withVolumeClaimTemplates(),
			),
			givenJsEnabled:   false,
			givenStorageType: fileStorageType,
			givenPVC:         true,
			wantStatefulSet:  nil,
			wantPVCDeletion:  true,
		},
		{
			name: "Should delete the StatefulSet and all the PVCs if Nats Jetstream is disabled and there are still PVCs left",
			givenStatefulSet: newStatefulSet(
				withReplicas(2),
				withVolumeClaimTemplates(),
			),
			givenJsEnabled:   false,
			givenStorageType: fileStorageType,
			wantStatefulSet:  nil,
			wantPVCDeletion:  true,
		},
	}

	setup := func(t *testing.T, jsEnabled bool, storageType string) (kubernetes.Interface, handleToggleJSFileStorage, *service.ActionContext) {
		k8sClient := fake.NewSimpleClientset()
		mockClient := mocks.Client{}
		mockClient.On("Clientset").Return(k8sClient, nil)
		action := handleToggleJSFileStorage{
			kubeClientProvider: func(context *service.ActionContext, logger *zap.SugaredLogger) (k8s.Client, error) {
				return &mockClient, nil
			},
		}

		chartProvider := &chartmocks.Provider{}
		chartValuesYAML := getJestreamValuesYAML(jsEnabled, storageType)
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

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			var err error
			k8sClient, action, actionContext := setup(t, tc.givenJsEnabled, tc.givenStorageType)

			if tc.givenStatefulSet != nil {
				_, err = createStatefulSet(actionContext.Context, k8sClient, tc.givenStatefulSet)
				require.NoError(t, err)
			}

			if tc.givenStatefulSet != nil && tc.givenPVC {
				// create PVCs according to replicas number
				replicas := int(*tc.givenStatefulSet.Spec.Replicas)
				if replicas > 0 {
					for i := 0; i < replicas; i++ {
						err := createPVC(actionContext.Context, k8sClient, pvcName+fmt.Sprint(i))
						require.NoError(t, err)
					}
					list, err := k8sClient.CoreV1().PersistentVolumeClaims(namespace).List(actionContext.Context, metav1.ListOptions{})
					require.NoError(t, err)
					assert.Equal(t, replicas, len(list.Items))
				}
			}

			// when
			err = action.Execute(actionContext, log.ContextLogger(actionContext, log.WithAction(actionName)))
			require.NoError(t, err)

			// then
			gotStatefulSet, err := k8sClient.AppsV1().StatefulSets(namespace).Get(actionContext.Context, statefulSetName, metav1.GetOptions{})
			if !k8serrors.IsNotFound(err) {
				require.NoError(t, err)
			}
			require.Equal(t, tc.wantStatefulSet, gotStatefulSet)

			if tc.wantPVCDeletion {
				// check if all the PVC instances were deleted
				list, err := k8sClient.CoreV1().PersistentVolumeClaims(namespace).List(actionContext.Context, metav1.ListOptions{})
				require.NoError(t, err)
				assert.Equal(t, 0, len(list.Items))
			}
		})
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

func withReplicas(replicas int) statefulSetOpt {
	return func(statefulSet *appsv1.StatefulSet) {
		ssReplicas := int32(replicas)
		statefulSet.Spec.Replicas = &ssReplicas
	}
}

func withVolumeClaimTemplates() statefulSetOpt {
	return func(statefulSet *appsv1.StatefulSet) {
		var persistentVolumeClaims []v1.PersistentVolumeClaim
		pvc := v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      volumeClaimName,
				Namespace: namespace,
			},
		}
		persistentVolumeClaims = append(persistentVolumeClaims, pvc)
		statefulSet.Spec.VolumeClaimTemplates = persistentVolumeClaims
	}
}

func getJestreamValuesYAML(jsEnabled bool, fileStorage string) string {
	return fmt.Sprintf(`
    global:
      jetstream:
        enabled: %s
        storage: %s`,
		fmt.Sprint(jsEnabled),
		fileStorage,
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
