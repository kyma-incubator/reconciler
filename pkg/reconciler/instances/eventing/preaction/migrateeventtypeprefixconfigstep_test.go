package preaction

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/log"
	"testing"

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

func TestEventingReconcilerPreAction(t *testing.T) {
	const (
		valueEmpty    = ""
		valueNotEmpty = "value"
	)

	testCases := []struct {
		name                      string
		givenPublisherDeployment  *appsv1.Deployment
		givenControllerDeployment *appsv1.Deployment
		wantPublisherDeployment   *appsv1.Deployment
		wantControllerDeployment  *appsv1.Deployment
	}{
		// no deployments found
		{
			name:                      "Should do nothing if Eventing deployments are not found",
			givenPublisherDeployment:  nil,
			givenControllerDeployment: nil,
			wantPublisherDeployment:   nil,
			wantControllerDeployment:  nil,
		},
		{
			name:                      "Should do nothing if Eventing deployment is already managed by the Kyma reconciler",
			givenPublisherDeployment:  newDeployment(publisherDeploymentName),
			givenControllerDeployment: newDeployment(controllerDeploymentName, withDeploymentLabels(map[string]string{managedByLabelKey: managedByLabelValue})),
			wantPublisherDeployment:   newDeployment(publisherDeploymentName),
			wantControllerDeployment:  newDeployment(controllerDeploymentName, withDeploymentLabels(map[string]string{managedByLabelKey: managedByLabelValue})),
		},
		// env value as an empty string
		{
			name:                      "Should delete Eventing deployments if publisher deployment env value is an empty string",
			givenPublisherDeployment:  newDeploymentWithEnvVarAsString(publisherDeploymentName, publisherDeploymentContainerName, publisherDeploymentEnvName, valueEmpty),
			givenControllerDeployment: newDeploymentWithEnvVarFromConfigMap(controllerDeploymentName, controllerDeploymentContainerName, controllerDeploymentEnvName, configMapName, configMapKey),
			wantPublisherDeployment:   nil,
			wantControllerDeployment:  nil,
		},
		{
			name:                      "Should delete Eventing deployments if publisher deployment env value is an empty string and controller deployment is not found",
			givenPublisherDeployment:  newDeploymentWithEnvVarAsString(publisherDeploymentName, publisherDeploymentContainerName, publisherDeploymentEnvName, valueEmpty),
			givenControllerDeployment: nil,
			wantPublisherDeployment:   nil,
			wantControllerDeployment:  nil,
		},
		{
			name:                      "Should delete Eventing deployments if controller deployment env value is an empty string",
			givenPublisherDeployment:  newDeploymentWithEnvVarFromConfigMap(publisherDeploymentName, publisherDeploymentContainerName, publisherDeploymentEnvName, configMapName, configMapKey),
			givenControllerDeployment: newDeploymentWithEnvVarAsString(controllerDeploymentName, controllerDeploymentContainerName, controllerDeploymentEnvName, valueEmpty),
			wantPublisherDeployment:   nil,
			wantControllerDeployment:  nil,
		},
		{
			name:                      "Should delete Eventing deployments if controller deployment env value is an empty string and publisher deployment is not found",
			givenPublisherDeployment:  nil,
			givenControllerDeployment: newDeploymentWithEnvVarAsString(controllerDeploymentName, controllerDeploymentContainerName, controllerDeploymentEnvName, valueEmpty),
			wantPublisherDeployment:   nil,
			wantControllerDeployment:  nil,
		},
		// env value as a non-empty string
		{
			name:                      "Should delete Eventing deployments if publisher deployment env value is a non-empty string",
			givenPublisherDeployment:  newDeploymentWithEnvVarAsString(publisherDeploymentName, publisherDeploymentContainerName, publisherDeploymentEnvName, valueNotEmpty),
			givenControllerDeployment: newDeploymentWithEnvVarFromConfigMap(controllerDeploymentName, controllerDeploymentContainerName, controllerDeploymentEnvName, configMapName, configMapKey),
			wantPublisherDeployment:   nil,
			wantControllerDeployment:  nil,
		},
		{
			name:                      "Should delete Eventing deployments if publisher deployment env value is a non-empty string and controller deployment is not found",
			givenPublisherDeployment:  newDeploymentWithEnvVarAsString(publisherDeploymentName, publisherDeploymentContainerName, publisherDeploymentEnvName, valueNotEmpty),
			givenControllerDeployment: nil,
			wantPublisherDeployment:   nil,
			wantControllerDeployment:  nil,
		},
		{
			name:                      "Should delete Eventing deployments if controller deployment env value is a non-empty string",
			givenPublisherDeployment:  newDeploymentWithEnvVarFromConfigMap(publisherDeploymentName, publisherDeploymentContainerName, publisherDeploymentEnvName, configMapName, configMapKey),
			givenControllerDeployment: newDeploymentWithEnvVarAsString(controllerDeploymentName, controllerDeploymentContainerName, controllerDeploymentEnvName, valueNotEmpty),
			wantPublisherDeployment:   nil,
			wantControllerDeployment:  nil,
		},
		{
			name:                      "Should delete Eventing deployments if controller deployment env value is a non-empty string and publisher deployment is not found",
			givenPublisherDeployment:  nil,
			givenControllerDeployment: newDeploymentWithEnvVarAsString(controllerDeploymentName, controllerDeploymentContainerName, controllerDeploymentEnvName, valueNotEmpty),
			wantPublisherDeployment:   nil,
			wantControllerDeployment:  nil,
		},
		// env value as a non-desired configmap key ref
		{
			name:                      "Should delete Eventing deployments if publisher deployment env value from configmap does not contain the desired values",
			givenPublisherDeployment:  newDeploymentWithEnvVarFromConfigMap(publisherDeploymentName, publisherDeploymentContainerName, publisherDeploymentEnvName, valueNotEmpty, valueNotEmpty),
			givenControllerDeployment: newDeploymentWithEnvVarFromConfigMap(controllerDeploymentName, controllerDeploymentContainerName, controllerDeploymentEnvName, configMapName, configMapKey),
			wantPublisherDeployment:   nil,
			wantControllerDeployment:  nil,
		},
		{
			name:                      "Should delete Eventing deployments if publisher deployment env value from configmap does not contain the desired values and controller deployment is not found",
			givenPublisherDeployment:  newDeploymentWithEnvVarFromConfigMap(publisherDeploymentName, publisherDeploymentContainerName, publisherDeploymentEnvName, valueNotEmpty, valueNotEmpty),
			givenControllerDeployment: nil,
			wantPublisherDeployment:   nil,
			wantControllerDeployment:  nil,
		},
		{
			name:                      "Should delete Eventing deployments if controller deployment env value from configmap does not contain the desired values",
			givenPublisherDeployment:  newDeploymentWithEnvVarFromConfigMap(publisherDeploymentName, publisherDeploymentContainerName, publisherDeploymentEnvName, configMapName, configMapKey),
			givenControllerDeployment: newDeploymentWithEnvVarFromConfigMap(controllerDeploymentName, controllerDeploymentContainerName, controllerDeploymentEnvName, valueNotEmpty, valueNotEmpty),
			wantPublisherDeployment:   nil,
			wantControllerDeployment:  nil,
		},
		{
			name:                      "Should delete Eventing deployments if controller deployment env value from configmap does not contain the desired values and publisher deployment is not found",
			givenPublisherDeployment:  nil,
			givenControllerDeployment: newDeploymentWithEnvVarFromConfigMap(controllerDeploymentName, controllerDeploymentContainerName, controllerDeploymentEnvName, valueNotEmpty, valueNotEmpty),
			wantPublisherDeployment:   nil,
			wantControllerDeployment:  nil,
		},
		// env value as a desired configmap key ref
		{
			name:                      "Should not delete Eventing deployments if env value from configmap contains the desired values",
			givenPublisherDeployment:  newDeploymentWithEnvVarFromConfigMap(publisherDeploymentName, publisherDeploymentContainerName, publisherDeploymentEnvName, configMapName, configMapKey),
			givenControllerDeployment: newDeploymentWithEnvVarFromConfigMap(controllerDeploymentName, controllerDeploymentContainerName, controllerDeploymentEnvName, configMapName, configMapKey),
			wantPublisherDeployment:   newDeploymentWithEnvVarFromConfigMap(publisherDeploymentName, publisherDeploymentContainerName, publisherDeploymentEnvName, configMapName, configMapKey),
			wantControllerDeployment:  newDeploymentWithEnvVarFromConfigMap(controllerDeploymentName, controllerDeploymentContainerName, controllerDeploymentEnvName, configMapName, configMapKey),
		},
	}

	setup := func() (kubernetes.Interface, migrateEventTypePrefixConfigStep, *service.ActionContext) {
		k8sClient := fake.NewSimpleClientset()
		action := migrateEventTypePrefixConfigStep{}
		mockClient := mocks.Client{}
		mockClient.On("Clientset").Return(k8sClient, nil)
		actionContext := &service.ActionContext{
			KubeClient: &mockClient,
			Context:    context.TODO(),
			Logger:     logger.NewLogger(false),
			Task:       &reconciler.Task{Version: "test"},
		}
		return k8sClient, action, actionContext
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			k8sClient, action, actionContext := setup()

			if tc.givenPublisherDeployment != nil {
				_, err = createDeployment(actionContext.Context, k8sClient, tc.givenPublisherDeployment)
				require.NoError(t, err)
			}

			if tc.givenControllerDeployment != nil {
				_, err = createDeployment(actionContext.Context, k8sClient, tc.givenControllerDeployment)
				require.NoError(t, err)
			}

			err = action.Execute(actionContext, log.ContextLogger(actionContext, log.WithAction(actionName)))
			require.NoError(t, err)

			gotPublisherDeployment, err := getDeployment(actionContext, k8sClient, publisherDeploymentName)
			require.NoError(t, err)
			require.Equal(t, tc.wantPublisherDeployment, gotPublisherDeployment)

			gotControllerDeployment, err := getDeployment(actionContext, k8sClient, controllerDeploymentName)
			require.NoError(t, err)
			require.Equal(t, tc.wantControllerDeployment, gotControllerDeployment)
		})
	}
}

type deploymentOpt func(*appsv1.Deployment)

func newDeployment(name string, opts ...deploymentOpt) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	for _, opt := range opts {
		opt(deployment)
	}

	return deployment
}

func withDeploymentContainer(name string, env []corev1.EnvVar) deploymentOpt {
	return func(deployment *appsv1.Deployment) {
		deployment.Spec.Template.Spec.Containers = []corev1.Container{
			{
				Name: name,
				Env:  env,
			},
		}
	}
}

func withDeploymentLabels(labels map[string]string) deploymentOpt {
	return func(deployment *appsv1.Deployment) {
		deployment.Labels = labels
	}
}

func newDeploymentWithEnvVarAsString(name, containerName, envName, envValue string) *appsv1.Deployment {
	return newDeployment(name, withDeploymentContainer(containerName, []corev1.EnvVar{{Name: envName, Value: envValue}}))
}

func newDeploymentWithEnvVarFromConfigMap(name, containerName, envName, configMapName, configMapKey string) *appsv1.Deployment {
	return newDeployment(name, withDeploymentContainer(containerName, []corev1.EnvVar{
		{
			Name: envName,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapName,
					},
					Key: configMapKey,
				},
			},
		},
	}))
}

func createDeployment(ctx context.Context, client kubernetes.Interface, deployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	return client.AppsV1().Deployments(deployment.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
}
