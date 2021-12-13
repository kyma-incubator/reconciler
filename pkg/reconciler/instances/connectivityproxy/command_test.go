package connectivityproxy

import (
	"context"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	chartmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/chart/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	serviceMocks "github.com/kyma-incubator/reconciler/pkg/reconciler/service/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	v1apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCommand(t *testing.T) {
	t.Run("Should copy required resources", func(t *testing.T) {
		expected := v1.Secret{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
			Immutable: nil,
			Data: map[string][]byte{
				"token": []byte("tokenValue"),
			},
			StringData: nil,
			Type:       "",
		}

		invoked := 0
		commands := CommandActions{
			targetClientSetFactory: func(context *service.ActionContext) (k8s.Interface, error) {
				return fake.NewSimpleClientset(), nil
			},
			clientSetFactory: func(logger *zap.SugaredLogger) (k8s.Interface, error) {
				return fake.NewSimpleClientset(&expected), nil
			},
			install: nil,
			copyFactory: []CopyFactory{
				func(task *reconciler.Task, inClusterClientSet, targetClientSet k8s.Interface) *SecretCopy {
					invoked++
					return &SecretCopy{
						Namespace:       "namespace",
						Name:            "name",
						targetClientSet: targetClientSet,
						from: &FromSecret{
							Namespace: "test-namespace",
							Name:      "test-name",
							inCluster: inClusterClientSet,
						},
					}
				},
				func(task *reconciler.Task, inClusterClientSet, targetClientSet k8s.Interface) *SecretCopy {
					invoked++
					return &SecretCopy{
						Namespace:       "namespace",
						Name:            "name",
						targetClientSet: targetClientSet,
						from: &FromSecret{
							Namespace: "test-namespace",
							Name:      "test-name",
							inCluster: inClusterClientSet,
						},
					}
				},
			},
		}

		client := mocks.Client{}
		client.On("Clientset").Return(fake.NewSimpleClientset(), nil)

		err := commands.CopyResources(&service.ActionContext{
			KubeClient:       &client,
			WorkspaceFactory: nil,
			Context:          nil,
			Logger:           nil,
			ChartProvider:    nil,
			Task:             &reconciler.Task{},
		})

		require.NoError(t, err)
		require.Equal(t, 2, invoked)
	})
}

func TestCommands(t *testing.T) {

	componentName := "connectivity-proxy"

	t.Run("Should invoke installation", func(t *testing.T) {
		// given
		commands := CommandActions{
			clientSetFactory:       nil,
			targetClientSetFactory: nil,
			install:                service.NewInstall(logger.NewLogger(true)),
			copyFactory:            nil,
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
		kubeClient.On("Deploy", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*service.LabelsInterceptor"), mock.AnythingOfType("*service.AnnotationsInterceptor"), mock.AnythingOfType("*service.ServicesInterceptor")).
			Return(nil, nil).Once()

		actionContext := &service.ActionContext{
			Context:       ctx,
			KubeClient:    kubeClient,
			Task:          &reconciler.Task{Component: componentName},
			ChartProvider: chartProvider,
			Logger:        logger.NewLogger(true),
		}

		component := &v1apps.StatefulSet{ObjectMeta: metav1.ObjectMeta{
			Name:   componentName,
			Labels: map[string]string{"release": "1.2.3"}},
		}

		// when
		err := commands.InstallOnReleaseChange(actionContext, component)

		// then
		require.NoError(t, err)
		kubeClient.AssertExpectations(t)
	})

	t.Run("Should skip installation if chart provider returned empty manifest", func(t *testing.T) {
		// given
		emptyManifest := ""

		commands := CommandActions{
			clientSetFactory:       nil,
			targetClientSetFactory: nil,
			install:                service.NewInstall(logger.NewLogger(true)),
			copyFactory:            nil,
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
		kubeClient.On("Deploy", ctx, emptyManifest, mock.AnythingOfType("string"), mock.AnythingOfType("*service.LabelsInterceptor"), mock.AnythingOfType("*service.AnnotationsInterceptor"), mock.AnythingOfType("*service.ServicesInterceptor")).
			Return(nil, nil).Once()

		actionContext := &service.ActionContext{
			Context:       ctx,
			KubeClient:    kubeClient,
			Task:          &reconciler.Task{Component: componentName},
			ChartProvider: chartProvider,
			Logger:        logger.NewLogger(true),
		}

		component := &v1apps.StatefulSet{ObjectMeta: metav1.ObjectMeta{
			Name:   componentName,
			Labels: map[string]string{"release": "1.2.3"}},
		}

		// when
		err := commands.InstallOnReleaseChange(actionContext, component)

		// then
		require.NoError(t, err)
	})

	t.Run("Should invoke installation if no app is installed", func(t *testing.T) {
		// given
		commands := CommandActions{
			clientSetFactory:       nil,
			targetClientSetFactory: nil,
			install:                service.NewInstall(logger.NewLogger(true)),
			copyFactory:            nil,
		}

		chartProvider := &chartmocks.Provider{}
		chartProvider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).
			Return(&chart.Manifest{
				Type:     chart.HelmChart,
				Name:     componentName,
				Manifest: cpManifest("1.2.3")}, nil)

		ctx := context.Background()
		kubeClient := &mocks.Client{}
		kubeClient.On("Deploy", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*service.LabelsInterceptor"), mock.AnythingOfType("*service.AnnotationsInterceptor"), mock.AnythingOfType("*service.ServicesInterceptor")).
			Return(nil, nil).Once()
		actionContext := &service.ActionContext{
			Context:       ctx,
			KubeClient:    kubeClient,
			Task:          &reconciler.Task{Component: componentName},
			ChartProvider: chartProvider,
			Logger:        logger.NewLogger(true),
		}

		// when
		err := commands.InstallOnReleaseChange(actionContext, nil)

		// then
		require.NoError(t, err)
		kubeClient.AssertExpectations(t)
	})

	t.Run("Should invoke installation if given set does not have release label", func(t *testing.T) {
		// given
		commands := CommandActions{
			clientSetFactory:       nil,
			targetClientSetFactory: nil,
			install:                service.NewInstall(logger.NewLogger(true)),
			copyFactory:            nil,
		}

		chartProvider := &chartmocks.Provider{}
		chartProvider.On("WithFilter", mock.AnythingOfType("chart.Filter")).
			Return(chartProvider)
		chartProvider.On("RenderManifest", mock.AnythingOfType("*chart.Component")).
			Return(&chart.Manifest{
				Type:     chart.HelmChart,
				Name:     componentName,
				Manifest: cpManifest("1.2.3")}, nil)

		ctx := context.Background()
		kubeClient := &mocks.Client{}
		kubeClient.On("Deploy", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*service.LabelsInterceptor"), mock.AnythingOfType("*service.AnnotationsInterceptor"), mock.AnythingOfType("*service.ServicesInterceptor")).
			Return(nil, nil).Once()
		actionContext := &service.ActionContext{
			Context:       ctx,
			KubeClient:    kubeClient,
			Task:          &reconciler.Task{Component: componentName},
			ChartProvider: chartProvider,
			Logger:        logger.NewLogger(true),
		}

		component := &v1apps.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: componentName}}

		// when
		err := commands.InstallOnReleaseChange(actionContext, component)

		// then
		require.NoError(t, err)
		kubeClient.AssertExpectations(t)
	})

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

		commands := CommandActions{
			clientSetFactory:       nil,
			targetClientSetFactory: nil,
			install:                delegateMock,
			copyFactory:            nil,
		}

		secret := &v1.Secret{Data: map[string][]byte{
			"key-1": []byte("value-1"),
			"key-2": []byte("value-2"),
		}}

		commands.PopulateConfigs(actionContext, secret)
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

		commands := CommandActions{
			clientSetFactory:       nil,
			targetClientSetFactory: nil,
			install:                delegateMock,
			copyFactory:            nil,
		}

		secret := &v1.Secret{Data: map[string][]byte{
			"parentkey": []byte(`{"key-1": "value-1","key-2": "value-2"}`),
		}}

		commands.PopulateConfigs(actionContext, secret)
		require.Equal(t, map[string]interface{}{
			"global.binding.key-1": "value-1",
			"global.binding.key-2": "value-2",
		}, actionContext.Task.Configuration)
	})
}

func TestCommandRemove(t *testing.T) {
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
		component := chart.NewComponentBuilder(task.Version, task.Component).
			WithNamespace(task.Namespace).
			WithProfile(task.Profile).
			WithConfiguration(task.Configuration).
			Build()

		provider := &chartmocks.Provider{}
		provider.On("RenderManifest", component).
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
		actionContext.KubeClient = client

		commands := CommandActions{
			clientSetFactory:       nil,
			targetClientSetFactory: nil,
			install:                nil,
			copyFactory:            nil,
		}

		err := commands.Remove(actionContext)
		require.NoError(t, err)
	})
}

func cpManifest(version string) string {
	return fmt.Sprintf("apiVersion: apps/v1\nkind: StatefulSet\nmetadata:\n  name: connectivity-proxy\n  labels:\n    release: \"%s\"\n", version)
}
