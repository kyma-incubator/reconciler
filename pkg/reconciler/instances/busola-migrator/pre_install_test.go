package busola_migrator

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/runtime/schema"
	restFake "k8s.io/client-go/rest/fake"
	"net/http"
	"path"
	"strings"
	"testing"
)

func TestPreInstall(t *testing.T) {
	//GIVEN
	vs := []VirtualSvcMeta{
		{
			Name:      "dex-virtualservice",
			Namespace: "kyma-system",
		},
		{
			Name:      "console-web",
			Namespace: "kyma-system",
		}}
	kubeconfig, err := ioutil.ReadFile("/Users/i515376/.kube/config")
	require.NoError(t, err)
	kubeClient, err := kubernetes.NewKubernetesClient(string(kubeconfig), true)
	require.NoError(t, err)
	p := NewVirtualServicePreInstallPatch(vs, "-old")

	clientSet, err := kubeClient.Clientset()
	require.NoError(t, err)

	//WHEN
	err = p.Run("", clientSet)

	//THE
	require.NoError(t, err)
}

func TestNewVirtualServicePreInstallPatch(t *testing.T) {
	//GIVEN
	name := "test"
	namespace := "test-namespace"
	vs := []VirtualSvcMeta{{Name: name, Namespace: namespace}}

	prefix := "-old"
	expectedPatch := virtualServicePatch{Spec: specPatch{Hosts: []string{"my-domain-old.kyma.io"}}}

	expectedCRPath := path.Join("/apis", "networking.istio.io/v1alpha3", "namespaces", namespace, "virtualservices", name)

	p := NewVirtualServicePreInstallPatch(vs, prefix)
	httpClient := restFake.CreateHTTPClient(func(request *http.Request) (*http.Response, error) {
		require.NotNil(t, request)
		switch request.Method {
		case http.MethodGet:
			{
				assertGet(t, request, expectedCRPath)
				resp := getVirtSvc(t)
				return resp, nil
			}
		case http.MethodPatch:
			{
				assertPatch(t, request, expectedPatch)
				resp := &http.Response{StatusCode: http.StatusNoContent}
				return resp, nil
			}
		default:
			{
				return nil, errors.New(fmt.Sprintf("Not supported method: %s", request.Method))
			}
		}
	})

	restClient := &restFake.RESTClient{
		NegotiatedSerializer: nil,
		GroupVersion:         schema.GroupVersion{},
		VersionedAPIPath:     "",
		Err:                  nil,
		Req:                  nil,
		Client:               httpClient,
		Resp:                 nil,
	}

	ctx := context.TODO()
	//WHEN
	err := p.patchVirtSvc(ctx, restClient, name, namespace)

	//THEN
	require.NoError(t, err)
}

func getVirtSvc(t *testing.T) *http.Response {
	out, err := ioutil.ReadFile("./test_files/virtual-service.yaml")
	require.NoError(t, err)

	reader := strings.NewReader(string(out))
	body := ioutil.NopCloser(reader)
	resp := &http.Response{Body: body}
	resp.StatusCode = http.StatusOK
	return resp
}

func assertGet(t *testing.T, request *http.Request, expectedPath string) {
	assert.Equal(t, expectedPath, request.URL.Path)

}
func assertPatch(t *testing.T, request *http.Request, expectedPatch virtualServicePatch) {
	out, err := io.ReadAll(request.Body)
	require.NoError(t, err)
	var currentPatch virtualServicePatch
	err = json.Unmarshal(out, &currentPatch)
	require.NoError(t, err)
	assert.Equal(t, expectedPatch, currentPatch)
}
