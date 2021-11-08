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

func TestIsDeploymentNotReady(t *testing.T) {
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
				ReadyReplicas: 0,
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
	require.False(t, ready)
}

func TestIsIgnoringOtherDeployments(t *testing.T) {
	ownedDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "kyma-system"},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "some-other-ns"},
	}

	replicaSets := []*appsv1.ReplicaSet{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "foo-123",
				Namespace:         "kyma-system",
				OwnerReferences:   []metav1.OwnerReference{*metav1.NewControllerRef(ownedDeployment, ownedDeployment.GroupVersionKind())},
				CreationTimestamp: metav1.NewTime(time.Now()),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "foo-456",
				Namespace:         "some-other-ns",
				OwnerReferences:   []metav1.OwnerReference{*metav1.NewControllerRef(deployment, deployment.GroupVersionKind())},
				CreationTimestamp: metav1.NewTime(time.Now().Add(2 * time.Second)),
			},
			Status: appsv1.ReplicaSetStatus{
				ReadyReplicas: 1,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "foo-789",
				Namespace:         "kyma-system",
				OwnerReferences:   []metav1.OwnerReference{*metav1.NewControllerRef(ownedDeployment, ownedDeployment.GroupVersionKind())},
				CreationTimestamp: metav1.NewTime(time.Now().Add(time.Second)),
			},
			Status: appsv1.ReplicaSetStatus{
				ReadyReplicas: 0,
			},
		},
	}

	var objects []runtime.Object
	objects = append(objects, deployment)
	objects = append(objects, ownedDeployment)
	for _, rs := range replicaSets {
		objects = append(objects, rs)
	}

	clientset := fake.NewSimpleClientset(objects...)

	ready, err := isDeploymentReady(context.Background(), clientset, &resource{name: "foo", namespace: "kyma-system"})

	require.NoError(t, err)
	require.False(t, ready)
}

func TestIsStatefulSetReady(t *testing.T) {
	tests := []struct {
		summary         string
		partition       int32
		replicas        int32
		updatedReplicas int32
		readyReplicas   int32
		expected        bool
	}{
		{summary: "all replicas scheduled and ready", partition: 0, replicas: 3, updatedReplicas: 3, readyReplicas: 3, expected: true},
		{summary: "replicas scheduled but not all ready", partition: 0, replicas: 3, updatedReplicas: 3, readyReplicas: 2, expected: false},
		{summary: "not all replicas scheduled", partition: 0, replicas: 3, updatedReplicas: 1, readyReplicas: 0, expected: false},
		{summary: "partitioned all replicas scheduled and ready", partition: 2, replicas: 5, updatedReplicas: 3, readyReplicas: 5, expected: true},
		{summary: "partitioned replicas scheduled but not all ready", partition: 1, replicas: 5, updatedReplicas: 4, readyReplicas: 4, expected: false},
		{summary: "partitioned not all replicas scheduled", partition: 1, replicas: 3, updatedReplicas: 1, readyReplicas: 0, expected: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.summary, func(t *testing.T) {
			t.Parallel()
			statefulSet := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "kyma-system"},
				Spec: appsv1.StatefulSetSpec{
					Replicas: &tc.replicas,
					UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
						RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
							Partition: &tc.partition,
						},
					},
				}, Status: appsv1.StatefulSetStatus{
					ReadyReplicas:   tc.readyReplicas,
					UpdatedReplicas: tc.updatedReplicas,
				},
			}

			c := fake.NewSimpleClientset(statefulSet)

			ready, err := isStatefulSetReady(context.Background(), c, &resource{name: "foo", namespace: "kyma-system"})

			require.NoError(t, err)
			require.Equal(t, tc.expected, ready)
		})
	}
}

func TestIsDaemonSetReady(t *testing.T) {
	tests := []struct {
		summary          string
		desiredScheduled int32
		numberReady      int32
		updatedScheduled int32
		expected         bool
	}{
		{summary: "all scheduled all ready", desiredScheduled: 3, numberReady: 3, updatedScheduled: 3, expected: true},
		{summary: "all scheduled one ready", desiredScheduled: 3, numberReady: 1, updatedScheduled: 3, expected: true},
		{summary: "all scheduled zero ready", desiredScheduled: 3, numberReady: 0, updatedScheduled: 3, expected: false},
		{summary: "scheduled mismatch", desiredScheduled: 1, numberReady: 3, updatedScheduled: 3, expected: false},
		{summary: "desired scheduled mismatch", desiredScheduled: 3, numberReady: 3, updatedScheduled: 1, expected: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.summary, func(t *testing.T) {
			t.Parallel()

			daemonSet := &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "kyma-system"},
				Status: appsv1.DaemonSetStatus{
					DesiredNumberScheduled: tc.desiredScheduled,
					NumberReady:            tc.numberReady,
					UpdatedNumberScheduled: tc.updatedScheduled,
				},
			}

			clientset := fake.NewSimpleClientset(daemonSet)

			ready, err := isDaemonSetReady(context.Background(), clientset, &resource{name: "foo", namespace: "kyma-system"})

			require.NoError(t, err)
			require.Equal(t, tc.expected, ready)
		})
	}
}
