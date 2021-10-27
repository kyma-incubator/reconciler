package actions

import (
	"encoding/json"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	clientsetmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/clientset/mocks"
	istioctlmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl/mocks"
	proxymocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/proxy/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/workspace"
	workspacemocks "github.com/kyma-incubator/reconciler/pkg/reconciler/workspace/mocks"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
)

const (
	istioManifest = `
apiVersion: version/v1
kind: Kind1
metadata:
  namespace: namespace
  name: name
---
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  namespace: namespace
  name: name
---
apiVersion: version/v2
kind: Kind2
metadata:
  namespace: namespace
  name: name
`
	istioctlMockSimpleVersion = `no running Istio pods in "istio-system"
{
	"clientVersion": {
	"version": "1.11.2",
	"revision": "revision",
	"golang_version": "go1.16.0",
	"status": "Clean",
	"tag": "1.11.2"
	}
}`

	istioctlMockCompleteVersion = `{
		"clientVersion": {
		  "version": "1.11.1",
		  "revision": "revision",
		  "golang_version": "go1.16.7",
		  "status": "Clean",
		  "tag": "1.11.1"
		},
		"meshVersion": [
		  {
			"Component": "pilot",
			"Info": {
			  "version": "1.11.1",
			  "revision": "revision",
			  "golang_version": "",
			  "status": "Clean",
			  "tag": "1.11.1"
			}
		  }
		],
		"dataPlaneVersion": [
		  {
			"ID": "id",
			"IstioVersion": "1.11.1"
		  }
		]
	  }`
)

func Test_DefaultIstioPerformer_Install(t *testing.T) {

	kubeConfig := "kubeConfig"
	log := logger.NewLogger(false)

	t.Run("should not install when istio operator could not be found in manifest", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		cmder.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(errors.New("istioctl error"))
		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(&cmder, &proxy, &provider)

		// when
		err := wrapper.Install(kubeConfig, "", log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Istio Operator definition could not be found")
		cmder.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	})

	t.Run("should not install Istio when istioctl returned an error", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		cmder.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(errors.New("istioctl error"))
		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(&cmder, &proxy, &provider)

		// when
		err := wrapper.Install(kubeConfig, istioManifest, log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "istioctl error")
		cmder.AssertCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	})

	t.Run("should install Istio when istioctl command was successful", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		cmder.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(nil)
		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(&cmder, &proxy, &provider)

		// when
		err := wrapper.Install(kubeConfig, istioManifest, log)

		// then
		require.NoError(t, err)
		cmder.AssertCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	})

}

func Test_DefaultIstioPerformer_Uninstall(t *testing.T) {
	kc := &mocks.Client{}
	kc.On("Kubeconfig").Return("kubeconfig")
	kc.On("Clientset").Return(fake.NewSimpleClientset(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "istio-system"},
	}), nil)
	log := logger.NewLogger(false)

	t.Run("should not uninstall Istio when istioctl returned an error", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		cmder.On("Uninstall", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(errors.New("istioctl error"))
		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		var wrapper IstioPerformer = NewDefaultIstioPerformer(&cmder, &proxy, &provider)

		// when
		err := wrapper.Uninstall(kc, log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "istioctl error")
		cmder.AssertCalled(t, "Uninstall", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	})

	t.Run("should uninstall Istio when istioctl command was successful", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		cmder.On("Uninstall", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(nil)
		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(&cmder, &proxy, &provider)

		// when
		err := wrapper.Uninstall(kc, log)

		// then
		require.NoError(t, err)
		cmder.AssertCalled(t, "Uninstall", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	})

}

func Test_DefaultIstioPerformer_PatchMutatingWebhook(t *testing.T) {

	log := logger.NewLogger(false)

	t.Run("should not patch MutatingWebhookConfiguration when kubeclient had returned an error", func(t *testing.T) {
		// given
		kubeClient := mocks.Client{}
		kubeClient.On("PatchUsingStrategy", "MutatingWebhookConfiguration", "istio-sidecar-injector", "istio-system", mock.Anything, types.JSONPatchType).Return(errors.New("kubeclient error"))
		cmder := istioctlmocks.Commander{}
		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(&cmder, &proxy, &provider)

		// when
		err := wrapper.PatchMutatingWebhook(&kubeClient, log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "kubeclient error")
	})

	t.Run("should patch MutatingWebhookConfiguration when kubeclient had not returned an error", func(t *testing.T) {
		// given
		kubeClient := mocks.Client{}
		kubeClient.On("PatchUsingStrategy", "MutatingWebhookConfiguration", "istio-sidecar-injector", "istio-system", mock.Anything, types.JSONPatchType).Return(nil)
		cmder := istioctlmocks.Commander{}
		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(&cmder, &proxy, &provider)

		// when
		err := wrapper.PatchMutatingWebhook(&kubeClient, log)

		// then
		require.NoError(t, err)
	})

}

func Test_DefaultIstioPerformer_Update(t *testing.T) {

	kubeConfig := "kubeConfig"
	log := logger.NewLogger(false)

	t.Run("should not update when istio operator could not be found in manifest", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		cmder.On("Upgrade", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(errors.New("istioctl error"))
		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(&cmder, &proxy, &provider)

		// when
		err := wrapper.Update(kubeConfig, "", log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Istio Operator definition could not be found")
		cmder.AssertNotCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	})

	t.Run("should not update Istio when istioctl returned an error", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		cmder.On("Upgrade", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(errors.New("istioctl error"))
		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(&cmder, &proxy, &provider)

		// when
		err := wrapper.Update(kubeConfig, istioManifest, log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "istioctl error")
		cmder.AssertCalled(t, "Upgrade", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	})

	t.Run("should update Istio when istioctl command was successful", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		cmder.On("Upgrade", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(nil)
		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(&cmder, &proxy, &provider)

		// when
		err := wrapper.Update(kubeConfig, istioManifest, log)

		// then
		require.NoError(t, err)
		cmder.AssertCalled(t, "Upgrade", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	})

}

func Test_extractIstioOperatorContextFrom(t *testing.T) {

	t.Run("should not extract istio operator from manifest that does not contain istio operator", func(t *testing.T) {
		// when
		result, err := extractIstioOperatorContextFrom("")

		// then
		require.Empty(t, result)
		require.Error(t, err)
		require.Contains(t, err.Error(), "could not be found")
	})

	t.Run("should extract istio operator from combo manifest", func(t *testing.T) {
		// when
		result, err := extractIstioOperatorContextFrom(istioManifest)

		// then
		require.NoError(t, err)
		require.Contains(t, result, "IstioOperator")
	})

}

func Test_DefaultIstioPerformer_ResetProxy(t *testing.T) {

	kubeConfig := "kubeconfig"
	log := logger.NewLogger(false)

	t.Run("should return error when kubeclient could not be retrieved", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		provider.On("RetrieveFrom", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(nil, errors.New("Kubeclient error"))
		wrapper := NewDefaultIstioPerformer(&cmder, &proxy, &provider)
		version := IstioVersion{
			ClientVersion:    "1.2.0",
			TargetVersion:    "1.2.0",
			PilotVersion:     "1.1.0",
			DataPlaneVersion: "1.1.0",
		}

		// when
		err := wrapper.ResetProxy(kubeConfig, version, log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Kubeclient error")
	})

	t.Run("should return error when istio proxy reset returned an error", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		proxy := proxymocks.IstioProxyReset{}
		proxy.On("Run", mock.Anything).Return(errors.New("Proxy reset error"))
		provider := clientsetmocks.Provider{}
		provider.On("RetrieveFrom", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(fake.NewSimpleClientset(), nil)
		wrapper := NewDefaultIstioPerformer(&cmder, &proxy, &provider)
		version := IstioVersion{
			ClientVersion:    "1.2.0",
			TargetVersion:    "1.2.0",
			PilotVersion:     "1.1.0",
			DataPlaneVersion: "1.1.0",
		}

		// when
		err := wrapper.ResetProxy(kubeConfig, version, log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Proxy reset error")
	})

	t.Run("should return no error when istio proxy reset was successful", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		proxy := proxymocks.IstioProxyReset{}
		proxy.On("Run", mock.Anything).Return(nil)
		provider := clientsetmocks.Provider{}
		provider.On("RetrieveFrom", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(fake.NewSimpleClientset(), nil)
		wrapper := NewDefaultIstioPerformer(&cmder, &proxy, &provider)
		version := IstioVersion{
			ClientVersion:    "1.2.0",
			TargetVersion:    "1.2.0",
			PilotVersion:     "1.1.0",
			DataPlaneVersion: "1.1.0",
		}

		// when
		err := wrapper.ResetProxy(kubeConfig, version, log)

		// then
		require.NoError(t, err)
	})

}

func Test_DefaultIstioPerformer_Version(t *testing.T) {

	kubeConfig := "kubeConfig"
	log := logger.NewLogger(false)

	t.Run("should not proceed if the version command output returns an empty string", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		factory := &workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&workspace.Workspace{ResourceDir: "../test_files"}, nil)
		cmder.On("Version", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return([]byte(""), nil)
		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(&cmder, &proxy, &provider)

		// when
		ver, err := wrapper.Version(factory, "version", "istio-test", kubeConfig, log)

		// then
		require.Empty(t, ver)
		require.Error(t, err)
		require.Contains(t, err.Error(), "command is empty")
	})

	t.Run("should not proceed if the targetVersion is not obtained", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		factory := &workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&workspace.Workspace{}, nil)
		cmder.On("Version", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return([]byte(""), nil)
		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(&cmder, &proxy, &provider)

		// when
		ver, err := wrapper.Version(factory, "version", "istio-test", kubeConfig, log)

		// then
		require.Empty(t, ver)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Target Version could not be obtained")
	})

	t.Run("should get only the client version when istio is not yet installed on the cluster", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		factory := &workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&workspace.Workspace{ResourceDir: "../test_files"}, nil)
		cmder.On("Version", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return([]byte(istioctlMockSimpleVersion), nil)
		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(&cmder, &proxy, &provider)

		// when
		ver, err := wrapper.Version(factory, "version", "istio-test", kubeConfig, log)

		// then
		require.EqualValues(t, IstioVersion{ClientVersion: "1.11.2", TargetVersion: "1.11.2"}, ver)
		require.NoError(t, err)
		cmder.AssertCalled(t, "Version", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		cmder.AssertNumberOfCalls(t, "Version", 1)
	})

	t.Run("should get all the expected versions when istio installed on the cluster", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		factory := &workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&workspace.Workspace{ResourceDir: "../test_files"}, nil)
		cmder.On("Version", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return([]byte(istioctlMockCompleteVersion), nil)
		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(&cmder, &proxy, &provider)

		// when
		ver, err := wrapper.Version(factory, "version", "istio-test", kubeConfig, log)

		// then
		require.EqualValues(t, IstioVersion{ClientVersion: "1.11.1", TargetVersion: "1.11.2", PilotVersion: "1.11.1", DataPlaneVersion: "1.11.1"}, ver)
		require.NoError(t, err)
		cmder.AssertCalled(t, "Version", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		cmder.AssertNumberOfCalls(t, "Version", 1)
	})
}

func TestGetTargetVersionFromChart(t *testing.T) {
	branch := "branch"

	t.Run("should not get target version when the workspace is not resolved", func(t *testing.T) {
		// given
		istioChart := "istio-test"
		factory := &workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&workspace.Workspace{}, nil)

		// when
		_, err := getTargetVersionFromChart(factory, branch, istioChart)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "no such file or directory")
	})

	t.Run("should not get target version when the istio Chart does not exist", func(t *testing.T) {
		// given
		istioChart := "istio-config"
		factory := &workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&workspace.Workspace{ResourceDir: "../test_files"}, nil)

		// when
		_, err := getTargetVersionFromChart(factory, branch, istioChart)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "no such file or directory")
	})

	t.Run("should get target version when Chart.yml is resolved", func(t *testing.T) {
		// given
		istioChart := "istio-test"
		factory := &workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&workspace.Workspace{ResourceDir: "../test_files"}, nil)

		// when
		targetVersion, err := getTargetVersionFromChart(factory, branch, istioChart)

		// then
		require.NoError(t, err)
		require.EqualValues(t, "1.11.2", targetVersion)
	})
}

func TestMapVersionToStruct(t *testing.T) {

	t.Run("Empty byte array for version coomand returns an error", func(t *testing.T) {
		// given
		versionOutput := []byte("")
		targetVersion := "targetVersion"

		// when
		_, err := mapVersionToStruct(versionOutput, targetVersion)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "command is empty")
	})

	t.Run("If unmarshalled properly, the byte array must be converted to struct", func(t *testing.T) {
		// given
		versionOutput := []byte(istioctlMockCompleteVersion)
		targetVersion := "targetVersion"
		expectedStruct := IstioVersion{
			ClientVersion:    "1.11.1",
			TargetVersion:    targetVersion,
			PilotVersion:     "1.11.1",
			DataPlaneVersion: "1.11.1",
		}

		// when
		gotStruct, err := mapVersionToStruct(versionOutput, targetVersion)

		// then
		require.NoError(t, err)
		require.EqualValues(t, expectedStruct, gotStruct)
	})

}

func TestGetVersionFromJSON(t *testing.T) {
	t.Run("should get all the expected versions when istio installed on the cluster", func(t *testing.T) {
		// given
		var version IstioVersionOutput
		err := json.Unmarshal([]byte(istioctlMockCompleteVersion), &version)

		// when
		gotClient := getVersionFromJSON("client", version)
		gotPilot := getVersionFromJSON("pilot", version)
		gotDataPlane := getVersionFromJSON("dataPlane", version)
		gotNothing := getVersionFromJSON("", version)

		// then
		require.NoError(t, err)
		require.Equal(t, "1.11.1", gotClient)
		require.Equal(t, "1.11.1", gotPilot)
		require.Equal(t, "1.11.1", gotDataPlane)
		require.Equal(t, "", gotNothing)

	})
}
