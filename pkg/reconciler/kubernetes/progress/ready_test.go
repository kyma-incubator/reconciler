package progress

import (
	"context"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
	"time"
)

func TestIsDeploymentReady(t *testing.T) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "kyma-system"},
	}

	replicaSets := []*appsv1.ReplicaSet{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "foo-123",
				Namespace:         "kyma-system",
				OwnerReferences:   []metav1.OwnerReference{*metav1.NewControllerRef(deployment, deployment.GroupVersionKind())},
				CreationTimestamp: metav1.NewTime(time.Now()),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "foo-456",
				Namespace:         "kyma-system",
				OwnerReferences:   []metav1.OwnerReference{*metav1.NewControllerRef(deployment, deployment.GroupVersionKind())},
				CreationTimestamp: metav1.NewTime(time.Now().Add(time.Second)),
			},
			Status: appsv1.ReplicaSetStatus{
				ReadyReplicas: 1,
			},
		},
	}

	var objects []runtime.Object
	objects = append(objects, deployment)
	for _, rs := range replicaSets {
		objects = append(objects, rs)
	}

	clientset := fake.NewSimpleClientset(objects...)

	ready, err := isDeploymentReady(context.Background(), clientset, &resource{name: "foo", namespace: "kyma-system"})

	require.NoError(t, err)
	require.True(t, ready)
}
