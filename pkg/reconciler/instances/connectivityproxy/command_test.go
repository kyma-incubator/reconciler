package connectivityproxy

import (
	"context"
	"fmt"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	connectivityclient "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/connectivityproxy/connectivityclient/mocks"
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	chartmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/chart/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	serviceMocks "github.com/kyma-incubator/reconciler/pkg/reconciler/service/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCommans_CreateCARootSecret(t *testing.T) {

	t.Run("Should create CA secret with CA string in specified namespace using values from configuration", func(t *testing.T) {
		commands := CommandActions{
			install: nil,
		}

		connCAClient := &connectivityclient.ConnectivityClient{}
		connCAClient.On("GetCA").Return([]byte("cacert-value"), nil)

		fakeClientSet := fake.NewSimpleClientset()

		k8Sclient := mocks.Client{}
		k8Sclient.On("Clientset").Return(fakeClientSet, nil)

		err := commands.CreateCARootSecret(&service.ActionContext{
			KubeClient:       &k8Sclient,
			WorkspaceFactory: nil,
			Context:          nil,
			Logger:           nil,
			ChartProvider:    nil,
			Task: &reconciler.Task{
				Configuration: map[string]interface{}{
					"istio.secret.name":      "test-name",
					"istio.secret.namespace": "test-namespace",
					"istio.secret.key":       "ca-cert",
				},
			},
		}, connCAClient)

		require.NoError(t, err)

		secret, err := fakeClientSet.CoreV1().Secrets("test-namespace").Get(context.Background(), "test-name", metav1.GetOptions{})

		require.NoError(t, err)

		value, ok := secret.Data["ca-cert"]

		require.NoError(t, err)
		require.Equal(t, true, ok)
		require.Equal(t, []byte("cacert-value"), value)
		connCAClient.AssertExpectations(t)
	})

	t.Run("Should return error when failed to read CA data from ConnectivityClient", func(t *testing.T) {
		commands := CommandActions{
			install: nil,
		}

		connCAClient := &connectivityclient.ConnectivityClient{}
		connCAClient.On("GetCA").Return([]byte{}, errors.New("some error"))

		fakeClientSet := fake.NewSimpleClientset()

		k8Sclient := mocks.Client{}
		k8Sclient.On("Clientset").Return(fakeClientSet, nil)

		err := commands.CreateCARootSecret(&service.ActionContext{
			KubeClient:       &k8Sclient,
			WorkspaceFactory: nil,
			Context:          nil,
			Logger:           nil,
			ChartProvider:    nil,
			Task: &reconciler.Task{
				Configuration: map[string]interface{}{
					"istio.secret.name":      "test-name",
					"istio.secret.namespace": "test-namespace",
					"istio.secret.key":       "ca-cert",
				},
			},
		}, connCAClient)

		require.Error(t, err)
		connCAClient.AssertExpectations(t)
	})

	t.Run("Should return error when CA secret name is missing in configuration", func(t *testing.T) {
		commands := CommandActions{
			install: nil,
		}

		connCAClient := &connectivityclient.ConnectivityClient{}
		connCAClient.On("GetCA").Return([]byte("cacert-value"), nil)

		fakeClientSet := fake.NewSimpleClientset()

		k8Sclient := mocks.Client{}
		k8Sclient.On("Clientset").Return(fakeClientSet, nil)

		err := commands.CreateCARootSecret(&service.ActionContext{
			KubeClient:       &k8Sclient,
			WorkspaceFactory: nil,
			Context:          nil,
			Logger:           nil,
			ChartProvider:    nil,
			Task: &reconciler.Task{
				Configuration: map[string]interface{}{
					"istio.secret.namespace": "test-namespace",
					"istio.secret.key":       "ca-cert",
				},
			},
		}, connCAClient)

		require.Error(t, err)
		connCAClient.AssertExpectations(t)
	})
}

func TestCommands_Apply(t *testing.T) {
	t.Setenv("GIT_CLONE_TOKEN", "token")
	componentName := "connectivity-proxy"

	t.Run("Should refresh existing installation", func(t *testing.T) {
		// given
		commands := CommandActions{
			install: service.NewInstall(logger.NewLogger(true)),
		}

		chartProvider := &chartmocks.Provider{}
		chartProvider.On("WithFilter", mock.AnythingOfType("chart.Filter")).
			Return(chartProvider)
		chartProvider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).
			Return(&chart.Manifest{
				Type:     chart.HelmChart,
				Name:     componentName,
				Manifest: cpManifest("1.2.4")}, nil)
		ctx := context.Background()
		kubeClient := &mocks.Client{}
		kubeClient.On("Deploy", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("string"),
			mock.AnythingOfType("*service.LabelsInterceptor"),
			mock.AnythingOfType("*service.AnnotationsInterceptor"),
			mock.AnythingOfType("*service.ServicesInterceptor"),
			mock.AnythingOfType("*service.ClusterWideResourceInterceptor"),
			mock.AnythingOfType("*service.NamespaceInterceptor"),
			mock.AnythingOfType("*service.FinalizerInterceptor")).
			Return(nil, nil).Once()

		actionContext := &service.ActionContext{
			Context:       ctx,
			KubeClient:    kubeClient,
			Task:          &reconciler.Task{Component: componentName},
			ChartProvider: chartProvider,
			Logger:        logger.NewLogger(true),
		}

		// when
		err := commands.Apply(actionContext, true)

		// then
		require.NoError(t, err)
		kubeClient.AssertExpectations(t)
	})

	t.Run("Should skip installation if chart provider returned empty manifest", func(t *testing.T) {
		// given
		emptyManifest := ""

		commands := CommandActions{
			install: service.NewInstall(logger.NewLogger(true)),
		}

		chartProvider := &chartmocks.Provider{}
		chartProvider.On("WithFilter", mock.AnythingOfType("chart.Filter")).
			Return(chartProvider)
		chartProvider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).
			Return(&chart.Manifest{
				Type:     chart.HelmChart,
				Name:     componentName,
				Manifest: emptyManifest}, nil)
		ctx := context.Background()
		kubeClient := &mocks.Client{}
		kubeClient.On("Deploy", ctx, emptyManifest, mock.AnythingOfType("string"),
			mock.AnythingOfType("*service.LabelsInterceptor"),
			mock.AnythingOfType("*service.AnnotationsInterceptor"),
			mock.AnythingOfType("*service.ServicesInterceptor"),
			mock.AnythingOfType("*service.ClusterWideResourceInterceptor"),
			mock.AnythingOfType("*service.NamespaceInterceptor"),
			mock.AnythingOfType("*service.FinalizerInterceptor")).
			Return(nil, nil).Once()

		actionContext := &service.ActionContext{
			Context:       ctx,
			KubeClient:    kubeClient,
			Task:          &reconciler.Task{Component: componentName},
			ChartProvider: chartProvider,
			Logger:        logger.NewLogger(true),
		}

		// when
		err := commands.Apply(actionContext, false)

		// then
		require.NoError(t, err)
	})

	t.Run("Should invoke installation if no app is installed", func(t *testing.T) {
		// given
		commands := CommandActions{
			install: service.NewInstall(logger.NewLogger(true)),
		}

		chartProvider := &chartmocks.Provider{}
		chartProvider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).
			Return(&chart.Manifest{
				Type:     chart.HelmChart,
				Name:     componentName,
				Manifest: cpManifest("1.2.3")}, nil)
		chartProvider.On("WithFilter", mock.AnythingOfType("chart.Filter")).
			Return(chartProvider)

		ctx := context.Background()
		kubeClient := &mocks.Client{}
		kubeClient.On("Deploy", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("string"),
			mock.AnythingOfType("*service.LabelsInterceptor"),
			mock.AnythingOfType("*service.AnnotationsInterceptor"),
			mock.AnythingOfType("*service.ServicesInterceptor"),
			mock.AnythingOfType("*service.ClusterWideResourceInterceptor"),
			mock.AnythingOfType("*service.NamespaceInterceptor"),
			mock.AnythingOfType("*service.FinalizerInterceptor")).
			Return(nil, nil).Once()
		actionContext := &service.ActionContext{
			Context:       ctx,
			KubeClient:    kubeClient,
			Task:          &reconciler.Task{Component: componentName},
			ChartProvider: chartProvider,
			Logger:        logger.NewLogger(true),
		}

		// when
		err := commands.Apply(actionContext, false)

		// then
		require.NoError(t, err)
		kubeClient.AssertExpectations(t)
	})
}

func TestCommands_PopulateConfig(t *testing.T) {

	t.Run("Should copy configuration from model", func(t *testing.T) {

		actionContext := &service.ActionContext{
			Context: context.Background(),
			Task: &reconciler.Task{
				Configuration: make(map[string]interface{}),
			},
		}

		delegateMock := &serviceMocks.Operation{}
		delegateMock.On("Invoke", actionContext.Context, nil,
			mock.AnythingOfType(fmt.Sprintf("%T", &reconciler.Task{})), // print the type of the object (*reconciler.Task)
			nil).
			Return(nil)

		secret := &v1.Secret{Data: map[string][]byte{
			"key-1": []byte("value-1"),
			"key-2": []byte("value-2"),
		}}

		populateConfigs(actionContext.Task.Configuration, secret)
		require.Equal(t, map[string]interface{}{
			"global.binding.key-1": "value-1",
			"global.binding.key-2": "value-2",
		}, actionContext.Task.Configuration)
	})

	t.Run("Should copy configuration with json inside", func(t *testing.T) {

		actionContext := &service.ActionContext{
			Context: context.Background(),
			Task: &reconciler.Task{
				Configuration: make(map[string]interface{}),
			},
		}

		delegateMock := &serviceMocks.Operation{}
		delegateMock.On("Invoke", actionContext.Context, nil,
			mock.AnythingOfType(fmt.Sprintf("%T", &reconciler.Task{})), // print the type of the object (*reconciler.Task)
			nil).
			Return(nil)

		secret := &v1.Secret{Data: map[string][]byte{
			"parentkey": []byte(`{"key-1": "value-1","key-2": "value-2"}`),
		}}

		populateConfigs(actionContext.Task.Configuration, secret)
		require.Equal(t, map[string]interface{}{
			"global.binding.key-1": "value-1",
			"global.binding.key-2": "value-2",
		}, actionContext.Task.Configuration)
	})

	t.Run("Should copy and flatten configuration with nested json inside one value", func(t *testing.T) {

		actionContext := &service.ActionContext{
			Context: context.Background(),
			Task: &reconciler.Task{
				Configuration: make(map[string]interface{}),
			},
		}

		delegateMock := &serviceMocks.Operation{}
		delegateMock.On("Invoke", actionContext.Context, nil,
			mock.AnythingOfType(fmt.Sprintf("%T", &reconciler.Task{})), // print the type of the object (*reconciler.Task)
			nil).
			Return(nil)

		secret := &v1.Secret{Data: map[string][]byte{
			"parentkey": []byte(`{"key-1": "value-1",  "key-2": "{\"key-3\":\"value-3\", \"key-4\":\"value-4\"}" }`),
		}}

		populateConfigs(actionContext.Task.Configuration, secret)
		require.Equal(t, map[string]interface{}{
			"global.binding.key-1": "value-1",
			"global.binding.key-3": "value-3",
			"global.binding.key-4": "value-4",
		}, actionContext.Task.Configuration)
	})
}

func TestCommandRemove(t *testing.T) {
	t.Setenv("GIT_CLONE_TOKEN", "token")

	t.Run("Should remove correct component", func(t *testing.T) {

		task := &reconciler.Task{
			ComponentsReady: nil,
			Component:       "test-component",
			Namespace:       "default",
			Version:         "test-version",
			Profile:         "test-profile",
			Configuration:   nil,
			Kubeconfig:      "",
			CallbackURL:     "",
			CorrelationID:   "",
			Repository:      nil,
			CallbackFunc:    nil,
		}

		provider := &chartmocks.Provider{}
		provider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).
			Return(&chart.Manifest{
				Type:     chart.HelmChart,
				Name:     task.Component,
				Manifest: "test-manifest",
			}, nil)

		actionContext := &service.ActionContext{
			KubeClient:       nil,
			WorkspaceFactory: nil,
			Context:          context.Background(),
			Logger:           nil,
			ChartProvider:    provider,
			Task:             task,
		}

		client := &mocks.Client{}
		client.On("Delete", actionContext.Context, "test-manifest", task.Namespace).
			Return(nil, nil)

		client.On("DeleteResource", actionContext.Context, "secret", "cc-certs", "istio-system").
			Return(nil, nil)

		client.On("DeleteResource", actionContext.Context, "secret", "cc-certs-cacert", "istio-system").
			Return(nil, nil)

		client.On("DeleteResource", actionContext.Context, "secret", mappingOperatorSecretName, kymaSystem).
			Return(nil, nil)

		client.On("DeleteResource", actionContext.Context, "configmap", mappingsConfigMap, kymaSystem).
			Return(nil, nil)

		actionContext.KubeClient = client

		commands := CommandActions{
			install: nil,
		}

		err := commands.Remove(actionContext)
		require.NoError(t, err)
		provider.AssertExpectations(t)
		client.AssertExpectations(t)
	})
}

func cpManifest(version string) string {
	return fmt.Sprintf("apiVersion: apps/v1\nkind: StatefulSet\nmetadata:\n  name: connectivity-proxy\n  labels:\n    release: \"%s\"\n", version)
}
