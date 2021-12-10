package hydra

import (
	"context"
	k8smocks "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	appv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/pointer"
	"testing"
	"time"
)

const (
	testNamespace = "kyma-system"
)

func Test_TriggerSynchronization(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	t.Parallel()
	t.Run("Should trigger synchronization when hydra-maester is behind hydra", func(t *testing.T) {
		// given
		hydraStartTimePod1 := time.Date(2021, 10, 10, 10, 10, 10, 10, time.UTC)
		hydraStartTimePod2 := time.Date(2021, 10, 10, 10, 10, 7, 10, time.UTC)
		hydraStartTimePod3 := time.Date(1900, 10, 10, 10, 10, 7, 10, time.UTC)
		hydraMasesterPodStartTime := time.Date(2021, 10, 10, 10, 10, 6, 10, time.UTC)
		kubeclient := fakeClient()
		addPod(kubeclient, "hydra1", "hydra", hydraStartTimePod1, t, v1.PodRunning)
		addPod(kubeclient, "hydra2", "hydra", hydraStartTimePod2, t, v1.PodRunning)
		addPod(kubeclient, "hydra3", "hydra", hydraStartTimePod3, t, v1.PodFailed)
		createDeployment(kubeclient, "ory-hydra-maester", hydraMasesterPodStartTime, t)
		addPod(kubeclient, "hydra-maester1", "hydra-maester", hydraMasesterPodStartTime, t, v1.PodRunning)
		client, err := kubeclient.Clientset()
		require.NoError(t, err)

		// when
		err = NewDefaultHydraSyncer().TriggerSynchronization(context.TODO(), client, logger, testNamespace)

		// then
		require.NoError(t, err)
		deployment, err2 := client.AppsV1().Deployments(testNamespace).Get(context.TODO(), "ory-hydra-maester", metav1.GetOptions{})
		require.NoError(t, err2)
		spec := deployment.Spec
		restartedAt := spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"]
		require.Greater(t, restartedAt, hydraMasesterPodStartTime.String())
	})

	t.Run("Should not trigger synchronization when hydra is behind hydra-maester", func(t *testing.T) {
		// given
		hydraStartTimePod1 := time.Date(2021, 10, 10, 10, 10, 10, 10, time.UTC)
		hydraStartTimePod2 := time.Date(2021, 10, 10, 10, 10, 7, 10, time.UTC)
		hydraStartTimePod3 := time.Date(2500, 10, 10, 10, 10, 6, 10, time.UTC)
		hydraMasesterPodStartTime := time.Date(2022, 10, 10, 10, 10, 6, 10, time.UTC)
		kubeclient := fakeClient()
		addPod(kubeclient, "hydra1", "hydra", hydraStartTimePod1, t, v1.PodRunning)
		addPod(kubeclient, "hydra2", "hydra", hydraStartTimePod2, t, v1.PodRunning)
		addPod(kubeclient, "hydra3", "hydra", hydraStartTimePod3, t, v1.PodPending)
		createDeployment(kubeclient, "ory-hydra-maester", hydraMasesterPodStartTime, t)
		addPod(kubeclient, "hydra-maester", "hydra-maester", hydraMasesterPodStartTime, t, v1.PodRunning)
		client, err := kubeclient.Clientset()
		require.NoError(t, err)

		// when
		err = NewDefaultHydraSyncer().TriggerSynchronization(context.TODO(), client, logger, testNamespace)

		// then
		require.NoError(t, err)
		deployment, err2 := client.AppsV1().Deployments(testNamespace).Get(context.TODO(), "ory-hydra-maester", metav1.GetOptions{})
		require.NoError(t, err2)
		spec := deployment.Spec
		restartedAt := spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"]
		// deployment should not have been restarted
		require.Equal(t, restartedAt, hydraMasesterPodStartTime.String())

	})
}
func Test_GetEarliestStartTime(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	t.Parallel()
	t.Run("Should find earliest pod starttime out of two", func(t *testing.T) {
		// given
		startTimePod1 := time.Date(2021, 10, 10, 10, 10, 10, 10, time.UTC)
		startTimePod2 := time.Date(2021, 10, 10, 10, 10, 7, 10, time.UTC)
		kubeclient := fakeClient()
		addPod(kubeclient, "hydra1", "hydra", startTimePod1, t, v1.PodRunning)
		addPod(kubeclient, "hydra2", "hydra", startTimePod2, t, v1.PodRunning)
		client, err := kubeclient.Clientset()
		require.NoError(t, err)

		// when
		earliestStartTime, err := getEarliestPodStartTime(context.TODO(), hydraPodName, client, logger, testNamespace)

		// then
		require.NoError(t, err)
		require.Equal(t, startTimePod2, earliestStartTime)
	})
	t.Run("Should determine earliest pod starttime when both started at the same time", func(t *testing.T) {
		// given
		startTime1 := time.Date(2021, 10, 10, 10, 10, 10, 10, time.UTC)
		startTime2 := time.Date(1900, 10, 10, 10, 10, 10, 10, time.UTC)
		kubeclient := fakeClient()
		addPod(kubeclient, "hydra1", "hydra", startTime1, t, v1.PodRunning)
		addPod(kubeclient, "hydra2", "hydra", startTime1, t, v1.PodRunning)
		addPod(kubeclient, "hydra3", "hydra", startTime2, t, v1.PodPending)
		client, err := kubeclient.Clientset()
		require.NoError(t, err)

		// when
		earliestStartTime, err := getEarliestPodStartTime(context.TODO(), hydraPodName, client, logger, testNamespace)

		// then
		require.NoError(t, err)
		require.Equal(t, earliestStartTime, startTime1)
	})

	t.Run("Should return error when no running pods found", func(t *testing.T) {
		// given
		startTime1 := time.Date(2021, 10, 10, 10, 10, 10, 10, time.UTC)
		startTime2 := time.Date(1900, 10, 10, 10, 10, 10, 10, time.UTC)
		kubeclient := fakeClient()
		addPod(kubeclient, "hydra1", "hydra", startTime1, t, v1.PodFailed)
		addPod(kubeclient, "hydra2", "hydra", startTime1, t, v1.PodPending)
		addPod(kubeclient, "hydra3", "hydra", startTime2, t, v1.PodPending)
		client, err := kubeclient.Clientset()
		require.NoError(t, err)

		// when
		_, err = getEarliestPodStartTime(context.TODO(), hydraPodName, client, logger, testNamespace)

		// then
		require.Error(t, err, "Could not find any running pod for label %s in namespace %s", hydraPodName, testNamespace)
	})
	t.Run("Should return error if no pods found", func(t *testing.T) {
		// given
		client, err := fakeClient().Clientset()
		require.NoError(t, err)

		// when
		_, err = getEarliestPodStartTime(context.TODO(), hydraPodName, client, logger, "kyma-system")

		// then
		require.Error(t, err, "Could not find pods for label %s in namespace %s", hydraPodName, testNamespace)
	})
}

func fakeClient() *k8smocks.Client {
	mockClient := &k8smocks.Client{}
	fakeClient := fake.NewSimpleClientset()
	mockClient.On("Clientset").Return(fakeClient, nil)
	mockClient.On("Kubeconfig").Return("kubeconfig")
	return mockClient
}

func addPod(client *k8smocks.Client, podName string, podLabel string, startTime time.Time, t *testing.T, podPhase v1.PodPhase) {
	fakeClient, _ := client.Clientset()
	nsMock := fakeClient.CoreV1().Pods("kyma-system")
	_, err := nsMock.Create(context.TODO(), &v1.Pod{
		TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:              podName,
			Namespace:         testNamespace,
			CreationTimestamp: metav1.NewTime(startTime),
			Labels:            map[string]string{"app.kubernetes.io/name": podLabel},
		},
		Status: v1.PodStatus{Phase: podPhase},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
}

func createDeployment(client *k8smocks.Client, deploymentName string, startTime time.Time, t *testing.T) {
	fakeClient, _ := client.Clientset()
	deplMock := fakeClient.AppsV1().Deployments(testNamespace)

	deploymentSpec := appv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{},
		Replicas: pointer.Int32Ptr(1),
		Template: v1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"kubectl.kubernetes.io/restartedAt": startTime.String()}},
			Spec:       v1.PodSpec{},
		},
	}
	_, err := deplMock.Create(context.TODO(), &appv1.Deployment{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{Name: deploymentName, Namespace: testNamespace, CreationTimestamp: metav1.NewTime(startTime)},
		Spec:       deploymentSpec,
		Status:     appv1.DeploymentStatus{}}, metav1.CreateOptions{})
	require.NoError(t, err)
}
