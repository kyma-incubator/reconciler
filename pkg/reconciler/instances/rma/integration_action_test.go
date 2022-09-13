package rma

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	testChartArchive        = "rmi-1.0.0.tgz"
	testUpgradeChartArchive = "rmi-1.1.0.tgz"
)

func Test_IntegrationAction_Run(t *testing.T) {
	testChart := fixChartArchive(t)
	testClient := NewFakeClient(fake.NewSimpleClientset())

	t.Run("should return error when rmi.chartUrl config is missing", func(t *testing.T) {
		// given
		action := NewIntegrationAction("test", NewFakeClient(nil))
		context := fixActionContext("")

		// when
		err := action.Run(context)

		// then
		require.Error(t, err)
	})

	t.Run("should return error when rmi.namespace config is missing", func(t *testing.T) {
		// given
		action := NewIntegrationAction("test", testClient)
		context := fixActionContext("fakeURL")
		delete(context.Task.Configuration, RmiNamespaceConfig)

		// when
		err := action.Run(context)

		// then
		require.Error(t, err)
	})

	t.Run("should install rmi when release not found", func(t *testing.T) {
		// given
		action := NewIntegrationAction("test", testClient)
		server := fixChartHTTPServer(t, testChart)
		context := fixActionContext(fixChartURL(server.URL))
		// should continue without rmi.vmalertGroupsNum
		delete(context.Task.Configuration, RmiVmalertGroupsNum)

		// when
		err := action.Run(context)

		// then
		require.NoError(t, err)
		rel, err := testClient.helmStorage.Last("test")
		assert.NoError(t, err)
		assert.Equal(t, release.StatusDeployed, rel.Info.Status)
		assert.Equal(t, 1, rel.Version)
		assertRMIConfig(t, context, 0, rel.Config)
		assertAuthCredentialOverrides(t, context)
	})

	t.Run("should not upgrade rmi when release found with same version", func(t *testing.T) {
		// given
		err := testClient.clientset.Tracker().Add(&v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vmuser-rmi-test",
				Namespace: "monitoring-system",
			},
			Data: map[string][]byte{
				"username": []byte("test"),
				"password": []byte("test"),
			},
		})
		require.NoError(t, err)
		action := NewIntegrationAction("test", testClient)
		server := fixChartHTTPServer(t, testChart)
		context := fixActionContext(fixChartURL(server.URL))

		// when
		err = action.Run(context)

		// then
		require.NoError(t, err)
		rel, err := testClient.helmStorage.Last("test")
		assert.NoError(t, err)
		assert.Equal(t, release.StatusDeployed, rel.Info.Status)
		assert.Equal(t, 1, rel.Version)
		assertAuthCredentialOverrides(t, context)
	})

	t.Run("should upgrade rmi when release found with different version", func(t *testing.T) {
		// given
		action := NewIntegrationAction("test", testClient)
		server := fixChartHTTPServer(t, testChart)
		context := fixActionContext(fmt.Sprintf("%s/%s", server.URL, testUpgradeChartArchive))

		// when
		err := action.Run(context)

		// then
		require.NoError(t, err)
		rel, err := testClient.helmStorage.Last("test")
		assert.NoError(t, err)
		assert.Equal(t, release.StatusDeployed, rel.Info.Status)
		assert.Equal(t, 2, rel.Version)
		assertRMIConfig(t, context, 2, rel.Config)
		assertAuthCredentialOverrides(t, context)
	})

	t.Run("should delete rmi when requested", func(t *testing.T) {
		// given
		action := NewIntegrationAction("test", testClient)
		server := fixChartHTTPServer(t, testChart)
		context := fixActionContext(fixChartURL(server.URL))
		context.Task.Type = model.OperationTypeDelete

		// when
		err := action.Run(context)

		// then
		require.NoError(t, err)
		_, err = testClient.helmStorage.Last("test")
		assert.Equal(t, driver.ErrReleaseNotFound, err)
	})

	t.Run("should ignore delete rmi request when not found", func(t *testing.T) {
		// given
		action := NewIntegrationAction("test", testClient)
		server := fixChartHTTPServer(t, testChart)
		context := fixActionContext(fixChartURL(server.URL))
		context.Task.Type = model.OperationTypeDelete

		// when
		err := action.Run(context)

		// then
		require.NoError(t, err)
		_, err = testClient.helmStorage.Last("test")
		assert.Equal(t, driver.ErrReleaseNotFound, err)
	})
}

func fixActionContext(chartURL string) *service.ActionContext {
	logger := logger.NewLogger(true)
	model := reconciler.Task{
		Component: "rma",
		Namespace: "kyma-system",
		Version:   "2.0.0",
		Profile:   "production",
		Configuration: map[string]interface{}{
			RmiNamespaceConfig:  "monitoring-system",
			RmiChartURLConfig:   chartURL,
			RmiVmalertGroupsNum: "6",
		},
		Metadata: keb.Metadata{
			GlobalAccountID: "testGA",
			SubAccountID:    "testSA",
			InstanceID:      "testInstance",
			Region:          "testRegion",
			ServiceID:       "testSID",
			ServicePlanID:   "testSPID",
			ServicePlanName: "testPlan",
			ShootName:       "test",
		},
		Type: model.OperationTypeReconcile,
	}

	mockClient := &mocks.Client{}
	mockClient.On("DeleteResource", mock.Anything, "deployment", "avs-bridge", "kyma-system").Return(nil, nil)

	return &service.ActionContext{
		Context:    context.Background(),
		Logger:     logger,
		Task:       &model,
		KubeClient: mockClient,
	}
}

func fixChartArchive(t *testing.T) []byte {
	buf := bytes.Buffer{}
	err := compress("./testdata", &buf)
	require.NoError(t, err)
	return buf.Bytes()
}

func fixChartURL(serverURL string) string {
	return fmt.Sprintf("%s/%s", serverURL, testChartArchive)
}

func fixChartHTTPServer(t *testing.T, chart []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write(chart)
		require.NoError(t, err)
	}))
}

func assertRMIConfig(t *testing.T, context *service.ActionContext, group int, config map[string]interface{}) {
	runtime := config["runtime"].(map[string]string)
	auth := config["auth"].(map[string]string)
	vmalert := config["vmalert"].(map[string]int)
	assert.Equal(t, context.Task.Metadata.InstanceID, runtime["instanceID"])
	assert.Equal(t, context.Task.Metadata.GlobalAccountID, runtime["globalAccountID"])
	assert.Equal(t, context.Task.Metadata.SubAccountID, runtime["subaccountID"])
	assert.Equal(t, context.Task.Metadata.ShootName, runtime["shootName"])
	assert.Equal(t, context.Task.Metadata.ServicePlanName, runtime["planName"])
	assert.Equal(t, context.Task.Metadata.Region, runtime["region"])
	assert.Equal(t, context.Task.Metadata.InstanceID, auth["username"])
	assert.NotEmpty(t, auth["password"])
	assert.Equal(t, group, vmalert["group"])
}

func assertAuthCredentialOverrides(t *testing.T, context *service.ActionContext) {
	assert.Equal(t, "testInstance", context.Task.Configuration["vmuser.username"])
	assert.NotEmpty(t, context.Task.Configuration["vmuser.password"])
}

func compress(src string, buf io.Writer) error {
	// tar > gzip > buf
	zr := gzip.NewWriter(buf)
	tw := tar.NewWriter(zr)

	// walk through every file in the folder
	err := filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// generate tar header
		header, err := tar.FileInfoHeader(fi, file)
		if err != nil {
			return err
		}

		header.Name = filepath.ToSlash(file)

		// write header
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		// if not a dir, write file content
		if !fi.IsDir() {
			data, err := os.Open(file)
			if err != nil {
				return err
			}
			if _, err := io.Copy(tw, data); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// produce tar
	if err := tw.Close(); err != nil {
		return err
	}
	// produce gzip
	if err := zr.Close(); err != nil {
		return err
	}
	//
	return nil
}
