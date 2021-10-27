package actions

import (
	"encoding/json"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/workspace"
	workspacemocks "github.com/kyma-incubator/reconciler/pkg/reconciler/workspace/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
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

func TestGetTargetVersionFromChart(t *testing.T) {
	branch := "branch"

	t.Run("should not get target version when the workspace is not resolved", func(t *testing.T) {
		//given
		istioChart := "istio-test"
		factory := &workspacemocks.Factory{}
		factory.On("Get", mock.AnythingOfType("string")).Return(&workspace.Workspace{}, nil)

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
		istioChart := "istio-test"
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

	t.Run("Empty byte array for version coomand returns an error", func(t *testing.T) {
		// given
		versionOutput := []byte("")
		targetVersion := "targetVersion"

		//when
		_, err := mapVersionToStruct(versionOutput, targetVersion)

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
		gotStruct, err := mapVersionToStruct(versionOutput, targetVersion)

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
