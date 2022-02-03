package actions

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	workspacemocks "github.com/kyma-incubator/reconciler/pkg/reconciler/chart/mocks"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	clientsetmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/clientset/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl"
	istioctlmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl/mocks"
	proxymocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/proxy/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
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

	t.Run("should not install when istio version could not be resolved", func(t *testing.T) {
		// given
		cmdResolver := TestCommanderResolver{err: errors.New("istioctl not found")}

		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(cmdResolver, &proxy, &provider)

		// when
		err := wrapper.Install(kubeConfig, "", "1.2.3", log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Istio Operator definition could not be found")
	})

	t.Run("should not install when istio operator could not be found in manifest", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		cmder.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(errors.New("istioctl error"))
		cmdResolver := TestCommanderResolver{cmder: &cmder}

		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(cmdResolver, &proxy, &provider)

		// when
		err := wrapper.Install(kubeConfig, "", "1.2.3", log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Istio Operator definition could not be found")
		cmder.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	})

	t.Run("should not install Istio when istioctl returned an error", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		cmder.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(errors.New("istioctl error"))

		cmdResolver := TestCommanderResolver{cmder: &cmder}
		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(cmdResolver, &proxy, &provider)

		// when
		err := wrapper.Install(kubeConfig, istioManifest, "1.2.3", log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "istioctl error")
		cmder.AssertCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	})

	t.Run("should install Istio when istioctl command was successful", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		cmder.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(nil)
		cmdResolver := TestCommanderResolver{cmder: &cmder}

		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(cmdResolver, &proxy, &provider)

		// when
		err := wrapper.Install(kubeConfig, istioManifest, "1.2.3", log)

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

	t.Run("should not uninstall when istio version could not be resolved", func(t *testing.T) {
		// given
		cmdResolver := TestCommanderResolver{err: errors.New("istioctl not found")}

		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		var wrapper IstioPerformer = NewDefaultIstioPerformer(cmdResolver, &proxy, &provider)

		// when
		err := wrapper.Uninstall(kc, "1.2.3", log)

		// then
		require.Error(t, err)
		require.Equal(t, "istioctl not found", err.Error())
	})

	t.Run("should not uninstall Istio when istioctl returned an error", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		cmder.On("Uninstall", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(errors.New("istioctl error"))
		cmdResolver := TestCommanderResolver{cmder: &cmder}

		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		var wrapper IstioPerformer = NewDefaultIstioPerformer(cmdResolver, &proxy, &provider)

		// when
		err := wrapper.Uninstall(kc, "1.2.3", log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "istioctl error")
		cmder.AssertCalled(t, "Uninstall", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	})

	t.Run("should uninstall Istio when istioctl command was successful", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		cmder.On("Uninstall", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(nil)
		cmdResolver := TestCommanderResolver{cmder: &cmder}

		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(cmdResolver, &proxy, &provider)

		// when
		err := wrapper.Uninstall(kc, "1.2.3", log)

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
		kubeClient.On("PatchUsingStrategy", context.TODO(), "MutatingWebhookConfiguration", "istio-sidecar-injector", "istio-system", mock.Anything, types.JSONPatchType).Return(errors.New("kubeclient error"))
		cmder := istioctlmocks.Commander{}
		cmdResolver := TestCommanderResolver{cmder: &cmder}

		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(&cmdResolver, &proxy, &provider)

		// when
		err := wrapper.PatchMutatingWebhook(context.TODO(), &kubeClient, log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "kubeclient error")
	})

	t.Run("should patch MutatingWebhookConfiguration when kubeclient had not returned an error", func(t *testing.T) {
		// given
		kubeClient := mocks.Client{}
		kubeClient.On("PatchUsingStrategy", context.TODO(), "MutatingWebhookConfiguration", "istio-sidecar-injector", "istio-system", mock.Anything, types.JSONPatchType).Return(nil)
		cmder := istioctlmocks.Commander{}
		cmdResolver := TestCommanderResolver{cmder: &cmder}

		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(cmdResolver, &proxy, &provider)

		// when
		err := wrapper.PatchMutatingWebhook(context.TODO(), &kubeClient, log)

		// then
		require.NoError(t, err)
	})

}

func Test_DefaultIstioPerformer_Update(t *testing.T) {

	kubeConfig := "kubeConfig"
	log := logger.NewLogger(false)

	t.Run("should not update when istio version could not be resolved", func(t *testing.T) {
		// given
		cmdResolver := TestCommanderResolver{err: errors.New("istioctl not found")}

		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(cmdResolver, &proxy, &provider)

		// when
		err := wrapper.Update(kubeConfig, "", "1.2.3", log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Istio Operator definition could not be found")
	})

	t.Run("should not update when istio operator could not be found in manifest", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		cmder.On("Upgrade", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(errors.New("istioctl error"))
		cmdResolver := TestCommanderResolver{cmder: &cmder}

		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(cmdResolver, &proxy, &provider)

		// when
		err := wrapper.Update(kubeConfig, "", "1.2.3", log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Istio Operator definition could not be found")
		cmder.AssertNotCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	})

	t.Run("should not update Istio when istioctl returned an error", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		cmder.On("Upgrade", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(errors.New("istioctl error"))
		cmdResolver := TestCommanderResolver{cmder: &cmder}

		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(cmdResolver, &proxy, &provider)

		// when
		err := wrapper.Update(kubeConfig, istioManifest, "1.2.3", log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "istioctl error")
		cmder.AssertCalled(t, "Upgrade", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	})

	t.Run("should update Istio when istioctl command was successful", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		cmder.On("Upgrade", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(nil)
		cmdResolver := TestCommanderResolver{cmder: &cmder}

		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(cmdResolver, &proxy, &provider)

		// when
		err := wrapper.Update(kubeConfig, istioManifest, "1.2.3", log)

		// then
		require.NoError(t, err)
		cmder.AssertCalled(t, "Upgrade", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	})

}

func Test_DefaultIstioPerformer_ResetProxy(t *testing.T) {

	kubeConfig := "kubeconfig"
	log := logger.NewLogger(false)
	ctx := context.Background()
	defer ctx.Done()
	t.Run("should return error when kubeclient could not be retrieved", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		cmdResolver := TestCommanderResolver{cmder: &cmder}

		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		provider.On("RetrieveFrom", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(nil, errors.New("Kubeclient error"))

		wrapper := NewDefaultIstioPerformer(cmdResolver, &proxy, &provider)
		proxyImageVersion := "1.2.0"

		// when
		err := wrapper.ResetProxy(ctx, kubeConfig, proxyImageVersion, log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Kubeclient error")
	})

	t.Run("should return error when istio proxy reset returned an error", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		cmdResolver := TestCommanderResolver{cmder: &cmder}

		proxy := proxymocks.IstioProxyReset{}
		proxy.On("Run", mock.Anything).Return(errors.New("Proxy reset error"))
		provider := clientsetmocks.Provider{}
		provider.On("RetrieveFrom", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(fake.NewSimpleClientset(), nil)

		wrapper := NewDefaultIstioPerformer(cmdResolver, &proxy, &provider)
		proxyImageVersion := "1.2.0"

		// when
		err := wrapper.ResetProxy(ctx, kubeConfig, proxyImageVersion, log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Proxy reset error")
	})

	t.Run("should return no error when istio proxy reset was successful", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		cmdResolver := TestCommanderResolver{cmder: &cmder}

		proxy := proxymocks.IstioProxyReset{}
		proxy.On("Run", mock.Anything).Return(nil)
		provider := clientsetmocks.Provider{}
		provider.On("RetrieveFrom", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(fake.NewSimpleClientset(), nil)

		wrapper := NewDefaultIstioPerformer(cmdResolver, &proxy, &provider)
		proxyImageVersion := "1.2.0"

		// when
		err := wrapper.ResetProxy(ctx, kubeConfig, proxyImageVersion, log)

		// then
		require.NoError(t, err)
	})

}

func Test_DefaultIstioPerformer_Version(t *testing.T) {

	kubeConfig := "kubeConfig"
	log := logger.NewLogger(false)

	t.Run("should not proceed if the istio version could not be resolved", func(t *testing.T) {
		// given
		factory := &workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&chart.KymaWorkspace{ResourceDir: "../test_files"}, nil)
		cmdResolver := TestCommanderResolver{err: errors.New("istioctl not found")}

		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(cmdResolver, &proxy, &provider)

		// when
		ver, err := wrapper.Version(factory, "version", "istio-test", kubeConfig, log)

		// then
		require.Empty(t, ver)
		require.Error(t, err)
		require.Equal(t, "istioctl not found", err.Error())
	})

	t.Run("should not proceed if the version command output returns an empty string", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		factory := &workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&chart.KymaWorkspace{ResourceDir: "../test_files"}, nil)
		cmder.On("Version", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return([]byte(""), nil)
		cmdResolver := TestCommanderResolver{cmder: &cmder}

		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(cmdResolver, &proxy, &provider)

		// when
		ver, err := wrapper.Version(factory, "version", "istio-test", kubeConfig, log)

		// then
		require.Empty(t, ver)
		require.Error(t, err)
		require.Contains(t, err.Error(), "command is empty")
	})

	t.Run("should not proceed if the targetVersion is not obtained", func(t *testing.T) {
		// given
		factory := &workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&chart.KymaWorkspace{}, nil)
		cmder := istioctlmocks.Commander{}
		cmder.On("Version", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return([]byte(""), nil)
		cmdResolver := TestCommanderResolver{cmder: &cmder}

		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(cmdResolver, &proxy, &provider)

		// when
		ver, err := wrapper.Version(factory, "version", "istio-test", kubeConfig, log)

		// then
		require.Empty(t, ver)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Target Version could not be obtained")
	})

	t.Run("should get only the client version when istio is not yet installed on the cluster", func(t *testing.T) {
		// given
		factory := &workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&chart.KymaWorkspace{ResourceDir: "../test_files"}, nil)
		cmder := istioctlmocks.Commander{}
		cmder.On("Version", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return([]byte(istioctlMockSimpleVersion), nil)
		cmdResolver := TestCommanderResolver{cmder: &cmder}

		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(cmdResolver, &proxy, &provider)

		// when
		ver, err := wrapper.Version(factory, "version", "istio-test", kubeConfig, log)

		// then
		require.EqualValues(t, IstioStatus{ClientVersion: "1.11.2", TargetVersion: "1.2.3-solo-fips-distroless"}, ver)
		require.NoError(t, err)
		cmder.AssertCalled(t, "Version", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		cmder.AssertNumberOfCalls(t, "Version", 1)
	})

	t.Run("should get all the expected versions when istio installed on the cluster", func(t *testing.T) {
		// given
		factory := &workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&chart.KymaWorkspace{ResourceDir: "../test_files"}, nil)
		cmder := istioctlmocks.Commander{}
		cmder.On("Version", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return([]byte(istioctlMockCompleteVersion), nil)
		cmdResolver := TestCommanderResolver{cmder: &cmder}
		proxy := proxymocks.IstioProxyReset{}
		provider := clientsetmocks.Provider{}
		wrapper := NewDefaultIstioPerformer(cmdResolver, &proxy, &provider)

		// when
		ver, err := wrapper.Version(factory, "version", "istio-test", kubeConfig, log)

		// then
		require.EqualValues(t, IstioStatus{ClientVersion: "1.11.1", TargetVersion: "1.2.3-solo-fips-distroless", PilotVersion: "1.11.1", DataPlaneVersion: "1.11.1"}, ver)
		require.NoError(t, err)
		cmder.AssertCalled(t, "Version", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
		cmder.AssertNumberOfCalls(t, "Version", 1)
	})
}

func Test_getTargetVersionFromPilotInChartValues(t *testing.T) {
	branch := "branch"

	t.Run("should not get target version when the workspace is not resolved", func(t *testing.T) {
		// given
		istioChart := "istio-test"
		factory := &workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&chart.KymaWorkspace{}, nil)

		// when
		_, err := getTargetVersionFromPilotInChartValues(factory, branch, istioChart)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "no such file or directory")
	})

	t.Run("should not get target version when the istio Chart does not exist", func(t *testing.T) {
		// given
		istioChart := "istio-config"
		factory := &workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&chart.KymaWorkspace{ResourceDir: "../test_files"}, nil)

		// when
		_, err := getTargetVersionFromPilotInChartValues(factory, branch, istioChart)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "no such file or directory")
	})

	t.Run("should get target version when values.yaml file is resolved", func(t *testing.T) {
		// given
		istioChart := "istio-test"
		factory := &workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&chart.KymaWorkspace{ResourceDir: "../test_files"}, nil)

		// when
		targetVersion, err := getTargetVersionFromPilotInChartValues(factory, branch, istioChart)

		// then
		require.NoError(t, err)
		require.EqualValues(t, "1.2.3-solo-fips-distroless", targetVersion)
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
		expectedStruct := IstioStatus{
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

type TestCommanderResolver struct {
	err   error
	cmder istioctl.Commander
}

func (tcr TestCommanderResolver) GetCommander(version istioctl.Version) (istioctl.Commander, error) {
	if tcr.err != nil {
		return nil, tcr.err
	}
	return tcr.cmder, nil
}
