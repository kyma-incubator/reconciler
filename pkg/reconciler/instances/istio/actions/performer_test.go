package actions

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	istioctlmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl/mocks"
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
	"revision": "96710172e1e47cee227e7e8dd591a318fdfe0326",
	"golang_version": "go1.16.7",
	"status": "Clean",
	"tag": "1.11.2"
	}
}`

	istioctlMockCompleteVersion = `{
		"clientVersion": {
		  "version": "1.11.1",
		  "revision": "ce6205d503e5c5e41af496ebbe01ece7dc6c3547",
		  "golang_version": "go1.16.7",
		  "status": "Clean",
		  "tag": "1.11.1"
		},
		"meshVersion": [
		  {
			"Component": "pilot",
			"Info": {
			  "version": "1.11.1",
			  "revision": "ce6205d503e5c5e41af496ebbe01ece7dc6c3547",
			  "golang_version": "",
			  "status": "Clean",
			  "tag": "1.11.1"
			}
		  }
		],
		"dataPlaneVersion": [
		  {
			"ID": "istio-ingressgateway-59ccd8f5-cpwxx.istio-system",
			"IstioVersion": "1.11.1"
		  }
		]
	  }`
)

func Test_DefaultIstioPerformer_Install(t *testing.T) {

	err := os.Setenv("ISTIOCTL_PATH", "path")
	require.NoError(t, err)
	kubeConfig := "kubeConfig"
	log := logger.NewLogger(false)

	t.Run("should not install when istio operator could not be found in manifest", func(t *testing.T) {
		// given
		err := os.Setenv("ISTIOCTL_PATH", "path")
		require.NoError(t, err)
		cmder := istioctlmocks.Commander{}
		cmder.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(errors.New("istioctl error"))
		wrapper := NewDefaultIstioPerformer(&cmder)

		// when
		err = wrapper.Install(kubeConfig, "", log)

		/// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Istio Operator definition could not be found")
		cmder.AssertNotCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	})

	t.Run("should not install Istio when istioctl returned an error", func(t *testing.T) {
		// given
		err := os.Setenv("ISTIOCTL_PATH", "path")
		require.NoError(t, err)
		cmder := istioctlmocks.Commander{}
		cmder.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(errors.New("istioctl error"))
		wrapper := NewDefaultIstioPerformer(&cmder)

		// when
		err = wrapper.Install(kubeConfig, istioManifest, log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "istioctl error")
		cmder.AssertCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	})

	t.Run("should install Istio when istioctl command was successful", func(t *testing.T) {
		// given
		err := os.Setenv("ISTIOCTL_PATH", "path")
		require.NoError(t, err)
		cmder := istioctlmocks.Commander{}
		cmder.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(nil)
		wrapper := NewDefaultIstioPerformer(&cmder)

		// when
		err = wrapper.Install(kubeConfig, istioManifest, log)

		// then
		require.NoError(t, err)
		cmder.AssertCalled(t, "Install", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	})

}

func Test_DefaultIstioPerformer_PatchMutatingWebhook(t *testing.T) {

	err := os.Setenv("ISTIOCTL_PATH", "path")
	require.NoError(t, err)
	log := logger.NewLogger(false)

	t.Run("should not patch MutatingWebhookConfiguration when kubeclient had returned an error", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		wrapper := NewDefaultIstioPerformer(&cmder)
		kubeClient := mocks.Client{}
		kubeClient.On("PatchUsingStrategy", "MutatingWebhookConfiguration", "istio-sidecar-injector", "istio-system", mock.Anything, types.JSONPatchType).Return(errors.New("kubeclient error"))

		// when
		err = wrapper.PatchMutatingWebhook(&kubeClient, log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "kubeclient error")
	})

	t.Run("should patch MutatingWebhookConfiguration when kubeclient had not returned an error", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		wrapper := NewDefaultIstioPerformer(&cmder)
		kubeClient := mocks.Client{}
		kubeClient.On("PatchUsingStrategy", "MutatingWebhookConfiguration", "istio-sidecar-injector", "istio-system", mock.Anything, types.JSONPatchType).Return(nil)

		// when
		err = wrapper.PatchMutatingWebhook(&kubeClient, log)

		// then
		require.NoError(t, err)
	})

}

func Test_DefaultIstioPerformer_Update(t *testing.T) {

	err := os.Setenv("ISTIOCTL_PATH", "path")
	require.NoError(t, err)
	kubeConfig := "kubeConfig"
	log := logger.NewLogger(false)

	t.Run("should not update when istio operator could not be found in manifest", func(t *testing.T) {
		// given
		err := os.Setenv("ISTIOCTL_PATH", "path")
		require.NoError(t, err)
		cmder := istioctlmocks.Commander{}
		cmder.On("Upgrade", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(errors.New("istioctl error"))
		wrapper := NewDefaultIstioPerformer(&cmder)

		// when
		err = wrapper.Update(kubeConfig, "", log)

		/// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Istio Operator definition could not be found")
		cmder.AssertNotCalled(t, "Update", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	})

	t.Run("should not update Istio when istioctl returned an error", func(t *testing.T) {
		// given
		err := os.Setenv("ISTIOCTL_PATH", "path")
		require.NoError(t, err)
		cmder := istioctlmocks.Commander{}
		cmder.On("Upgrade", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(errors.New("istioctl error"))
		wrapper := NewDefaultIstioPerformer(&cmder)

		// when
		err = wrapper.Update(kubeConfig, istioManifest, log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "istioctl error")
		cmder.AssertCalled(t, "Upgrade", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger"))
	})

	t.Run("should update Istio when istioctl command was successful", func(t *testing.T) {
		// given
		err := os.Setenv("ISTIOCTL_PATH", "path")
		require.NoError(t, err)
		cmder := istioctlmocks.Commander{}
		cmder.On("Upgrade", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(nil)
		wrapper := NewDefaultIstioPerformer(&cmder)

		// when
		err = wrapper.Update(kubeConfig, istioManifest, log)

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

func Test_DefaultIstioPerformer_Version(t *testing.T) {
	err := os.Setenv("ISTIOCTL_PATH", "path")
	require.NoError(t, err)
	kubeConfig := "kubeConfig"
	log := logger.NewLogger(false)

	t.Run("should not proceed if the version command output returns an empty string", func(t *testing.T) {
		// given
		cmder := istioctlmocks.Commander{}
		factory := &workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&workspace.Workspace{ResourceDir: "../test_files"}, nil)
		cmder.On("Version", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return([]byte(""), nil)
		wrapper := NewDefaultIstioPerformer(&cmder)

		// when
		ver, err := wrapper.Version(factory, "version", "istio-configuration-test", kubeConfig, log)

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
		wrapper := NewDefaultIstioPerformer(&cmder)

		// when
		ver, err := wrapper.Version(factory, "version", "istio-configuration-test", kubeConfig, log)

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
		wrapper := NewDefaultIstioPerformer(&cmder)

		// when
		ver, err := wrapper.Version(factory, "version", "istio-configuration-test", kubeConfig, log)

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
		wrapper := NewDefaultIstioPerformer(&cmder)

		// when
		ver, err := wrapper.Version(factory, "version", "istio-configuration-test", kubeConfig, log)

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
		//given
		istioChart := "istio-configuration-test"
		factory := &workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&workspace.Workspace{}, nil)
		//factory.On("Get", mock.AnythingOfType("string")).Return(&workspace.Workspace{ResourceDir: "../test_files"}, nil)

		//when
		_, err := getTargetVersionFromChart(factory, branch, istioChart)

		//then
		require.Error(t, err)
		require.Contains(t, err.Error(), "no such file or directory")
	})

	t.Run("should not get target version when the istio Chart does not exist", func(t *testing.T) {
		//given
		istioChart := "istio-config"
		factory := &workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&workspace.Workspace{ResourceDir: "../test_files"}, nil)

		//when
		_, err := getTargetVersionFromChart(factory, branch, istioChart)

		//then
		require.Error(t, err)
		require.Contains(t, err.Error(), "no such file or directory")
	})

	t.Run("should get target version when Chart.yml is resolved", func(t *testing.T) {
		//given
		istioChart := "istio-configuration-test"
		factory := &workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&workspace.Workspace{ResourceDir: "../test_files"}, nil)

		//when
		targetVersion, err := getTargetVersionFromChart(factory, branch, istioChart)

		//then
		require.NoError(t, err)
		require.EqualValues(t, "1.11.2", targetVersion)
	})
}

func TestMapVersionToStruct(t *testing.T) {
	log := logger.NewLogger(false)

	t.Run("Empty byte array for version coomand returns an error", func(t *testing.T) {
		// given
		versionOutput := []byte("")
		targetVersion := "targetVersion"

		//when
		_, err := mapVersionToStruct(versionOutput, targetVersion, log)

		//then
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

		//when
		gotStruct, err := mapVersionToStruct(versionOutput, targetVersion, log)

		//then
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
