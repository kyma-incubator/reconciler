package warden

import (
	"context"
	"fmt"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCleanupWardenAdmissionCertColumeMounts_Run(t *testing.T) {
	t.Run("no warden admission deployment found", func(t *testing.T) {
		ctx := context.Background()
		mockClient := &mocks.Client{}
		mockClient.On("GetDeployment", ctx, wardenAdmissionDeploymentName, wardenAdmissionDeploymentNamespace).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
		context := &service.ActionContext{
			Context:    ctx,
			Logger:     zap.NewNop().Sugar(),
			KubeClient: mockClient,
		}
		action := &CleanupWardenAdmissionCertColumeMounts{}
		err := action.Run(context)
		require.NoError(t, err)
	})

	t.Run("error while reading warden admission deployment", func(t *testing.T) {
		ctx := context.Background()
		mockClient := &mocks.Client{}
		mockClient.On("GetDeployment", ctx, wardenAdmissionDeploymentName, wardenAdmissionDeploymentNamespace).Return(nil, errors.NewBadRequest(""))
		context := &service.ActionContext{
			Context:    ctx,
			Logger:     zap.NewNop().Sugar(),
			KubeClient: mockClient,
		}
		action := &CleanupWardenAdmissionCertColumeMounts{}
		err := action.Run(context)
		require.Error(t, err)
	})

	t.Run("warden admission deployment found - doesnt qualify for cleanup", func(t *testing.T) {
		ctx := context.Background()
		mockClient := &mocks.Client{}
		mockClient.On("GetDeployment", ctx, wardenAdmissionDeploymentName, wardenAdmissionDeploymentNamespace).Return(fixedDeploymentWith("whatever", nil, nil), nil)
		context := &service.ActionContext{
			Context:    ctx,
			Logger:     zap.NewNop().Sugar(),
			KubeClient: mockClient,
		}
		action := &CleanupWardenAdmissionCertColumeMounts{}
		err := action.Run(context)
		require.NoError(t, err)
	})

	t.Run("warden admission deployment found in v 0.10.0 but no volumemounts", func(t *testing.T) {
		ctx := context.Background()
		mockClient := &mocks.Client{}
		mockClient.On("GetDeployment", ctx, wardenAdmissionDeploymentName, wardenAdmissionDeploymentNamespace).Return(fixedDeploymentWith("0.10.0", nil, nil), nil)
		context := &service.ActionContext{
			Context:    ctx,
			Logger:     zap.NewNop().Sugar(),
			KubeClient: mockClient,
		}
		action := &CleanupWardenAdmissionCertColumeMounts{}
		err := action.Run(context)
		require.NoError(t, err)
	})

	t.Run("warden admission deployment found in v 0.10.2 for cleanup with volumemounts", func(t *testing.T) {

		vms := []corev1.VolumeMount{
			{Name: "foo"}, {Name: "certs"},
		}
		vm := []corev1.Volume{
			{Name: "foo"}, {Name: "certs"},
		}

		ctx := context.Background()
		mockClient := &mocks.Client{}
		mockClient.On("GetDeployment", ctx, wardenAdmissionDeploymentName, wardenAdmissionDeploymentNamespace).Return(fixedDeploymentWith("0.10.2", vms, vm), nil)
		mockClient.On("PatchUsingStrategy", ctx, "Deployment", wardenAdmissionDeploymentName, wardenAdmissionDeploymentNamespace, []byte(`[{"op": "remove", "path": "/spec/template/spec/containers/0/volumeMounts/1"},{"op": "remove", "path": "/spec/template/spec/volumes/1"}]`), mock.Anything).Return(nil)
		context := &service.ActionContext{
			Context:    ctx,
			Logger:     zap.NewNop().Sugar(),
			KubeClient: mockClient,
		}
		action := &CleanupWardenAdmissionCertColumeMounts{}
		err := action.Run(context)
		require.NoError(t, err)
	})

	t.Run("warden admission deployment found in v 0.10.2 for cleanup with volumemounts - handle error", func(t *testing.T) {

		vms := []corev1.VolumeMount{
			{Name: "certs"}, {Name: "foo"},
		}
		vm := []corev1.Volume{
			{Name: "foo"}, {Name: "certs"},
		}

		ctx := context.Background()
		mockClient := &mocks.Client{}
		mockClient.On("GetDeployment", ctx, wardenAdmissionDeploymentName, wardenAdmissionDeploymentNamespace).Return(fixedDeploymentWith("0.10.2", vms, vm), nil)
		mockClient.On("PatchUsingStrategy", ctx, "Deployment", wardenAdmissionDeploymentName, wardenAdmissionDeploymentNamespace, []byte(`[{"op": "remove", "path": "/spec/template/spec/containers/0/volumeMounts/0"},{"op": "remove", "path": "/spec/template/spec/volumes/1"}]`), mock.Anything).Return(errors.NewBadRequest(""))
		context := &service.ActionContext{
			Context:    ctx,
			Logger:     zap.NewNop().Sugar(),
			KubeClient: mockClient,
		}
		action := &CleanupWardenAdmissionCertColumeMounts{}
		err := action.Run(context)
		require.Error(t, err)
	})
}

func fixedDeploymentWith(imageVersion string, volumeMounts []corev1.VolumeMount, volumes []corev1.Volume) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      wardenAdmissionDeploymentName,
			Namespace: wardenAdmissionDeploymentNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:         "admission",
							Image:        fmt.Sprintf("europe-docker.pkg.dev/kyma-project/prod/warden/admission:%s", imageVersion),
							VolumeMounts: volumeMounts,
						},
					},
					Volumes: volumes,
				},
			},
		},
	}
}
