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
	testNamespase = "kyma-system"
)

func Test_TriggerSynchronization(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	t.Parallel()
	t.Run("Should trigger synchronization if hydra-maester is behind hydra", func(t *testing.T) {
		// given
		hydraStartTimePod1 := time.Date(2021, 10, 10, 10, 10, 10, 10, time.UTC)
		hydraStartTimePod2 := time.Date(2021, 10, 10, 10, 10, 7, 10, time.UTC)
		hydraMasesterPodStartTime := time.Date(2021, 10, 10, 10, 10, 6, 10, time.UTC)
		kubeclient := fakeClient()
		addPod(kubeclient, "hydra1", "hydra", hydraStartTimePod1, t)
		addPod(kubeclient, "hydra2", "hydra", hydraStartTimePod2, t)
		createDeployment(kubeclient, "ory-hydra-maester", hydraMasesterPodStartTime, t)
		addPod(kubeclient, "hydra-maester1", "hydra-maester", hydraMasesterPodStartTime, t)
		client, _ := kubeclient.Clientset()

		// when
		err := NewDefaultHydraClient().TriggerSynchronization(context.TODO(), client, logger, "kyma-system")

		// then
		require.NoError(t, err)
		deployment, err2 := client.AppsV1().Deployments(testNamespase).Get(context.TODO(), "ory-hydra-maester", metav1.GetOptions{})
		require.NoError(t, err2)
		spec := deployment.Spec
		restartedAt := spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"]
		require.Greater(t, restartedAt, hydraMasesterPodStartTime.String())
	})

	t.Run("Should not trigger synchronization if hydra is behind hydra-maester", func(t *testing.T) {
		// given
		hydraStartTimePod1 := time.Date(2021, 10, 10, 10, 10, 10, 10, time.UTC)
		hydraStartTimePod2 := time.Date(2021, 10, 10, 10, 10, 7, 10, time.UTC)
		hydraMasesterPodStartTime := time.Date(2022, 10, 10, 10, 10, 6, 10, time.UTC)
		kubeclient := fakeClient()
		addPod(kubeclient, "hydra1", "hydra", hydraStartTimePod1, t)
		addPod(kubeclient, "hydra2", "hydra", hydraStartTimePod2, t)
		createDeployment(kubeclient, "ory-hydra-maester", hydraMasesterPodStartTime, t)
		addPod(kubeclient, "hydra-maester", "hydra-maester", hydraMasesterPodStartTime, t)
		client, _ := kubeclient.Clientset()

		// when
		deploymentOld, _ := client.AppsV1().Deployments("kyma-system").Get(context.TODO(), "ory-hydra-maester", metav1.GetOptions{})
		logger.Debugf("Deploy found ", deploymentOld.Name)
		err := NewDefaultHydraClient().TriggerSynchronization(context.TODO(), client, logger, testNamespase)

		// then
		require.NoError(t, err)
		deployment, err2 := client.AppsV1().Deployments(testNamespase).Get(context.TODO(), "ory-hydra-maester", metav1.GetOptions{})
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
		addPod(kubeclient, "hydra1", "hydra", startTimePod1, t)
		addPod(kubeclient, "hydra2", "hydra", startTimePod2, t)
		client, _ := kubeclient.Clientset()

		// when
		earliestStartTime, err := getEarliestPodStartTime(hydraPodName, context.TODO(), client, logger, testNamespase)

		// then
		require.NoError(t, err)
		require.Equal(t, startTimePod2, earliestStartTime)
	})
	t.Run("Should determine earliest pod starttime if both started at the same time", func(t *testing.T) {
		// given
		startTime1 := time.Date(2021, 10, 10, 10, 10, 10, 10, time.UTC)
		kubeclient := fakeClient()
		addPod(kubeclient, "hydra1", "hydra", startTime1, t)
		addPod(kubeclient, "hydra2", "hydra", startTime1, t)
		client, _ := kubeclient.Clientset()

		// when
		earliestStartTime, err := getEarliestPodStartTime(hydraPodName, context.TODO(), client, logger, "kyma-system")

		// then
		require.NoError(t, err)
		require.Equal(t, earliestStartTime, startTime1)
	})

	t.Run("Should return error if no pods found", func(t *testing.T) {
		// given
		client, _ := fakeClient().Clientset()

		// when
		_, err := getEarliestPodStartTime(hydraPodName, context.TODO(), client, logger, "kyma-system")

		// then
		require.Error(t, err, "Could not find pods for label %s in namespace %s", hydraPodName, testNamespase)
	})
}

func fakeClient() *k8smocks.Client {
	mockClient := &k8smocks.Client{}
	fakeClient := fake.NewSimpleClientset()
	mockClient.On("Clientset").Return(fakeClient, nil)
	mockClient.On("Kubeconfig").Return("kubeconfig")
	return mockClient
}

func addPod(client *k8smocks.Client, podName string, podLabel string, startTime time.Time, t *testing.T) {
	fakeClient, _ := client.Clientset()
	nsMock := fakeClient.CoreV1().Pods("kyma-system")
	_, err := nsMock.Create(context.TODO(), &v1.Pod{
		TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:              podName,
			Namespace:         testNamespase,
			CreationTimestamp: metav1.NewTime(startTime),
			Labels:            map[string]string{"app.kubernetes.io/name": podLabel},
		}}, metav1.CreateOptions{})
	require.NoError(t, err)
}

func createDeployment(client *k8smocks.Client, deploymentName string, startTime time.Time, t *testing.T) {
	fakeClient, _ := client.Clientset()
	deplMock := fakeClient.AppsV1().Deployments(testNamespase)

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
		ObjectMeta: metav1.ObjectMeta{Name: deploymentName, Namespace: testNamespase, CreationTimestamp: metav1.NewTime(startTime)},
		Spec:       deploymentSpec,
		Status:     appv1.DeploymentStatus{}}, metav1.CreateOptions{})
	require.NoError(t, err)
}
