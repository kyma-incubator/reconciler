package busola_migrator_test

import (
	"fmt"
	busola_migrator "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/busola-migrator"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/runtime/schema"
	restFake "k8s.io/client-go/rest/fake"
	"net/http"
	"strings"
	"testing"
)

func TestPreInstall(t *testing.T) {
	//GIVEN
	vs := []busola_migrator.VirtualSvcMeta{
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
	p := busola_migrator.NewVirtualServicePreInstallPatch(vs, "-old")

	clientSet, err := kubeClient.Clientset()
	require.NoError(t, err)

	//WHEN
	err = p.Run("", clientSet)

	//THE
	require.NoError(t, err)
}

func TestNewVirtualServicePreInstallPatch(t *testing.T) {
	//GIVEN
	vs := []busola_migrator.VirtualSvcMeta{
		{
			Name:      "test",
			Namespace: "test-namespace",
		}}
	prefix := "-old"
	p := busola_migrator.NewVirtualServicePreInstallPatch(vs, prefix)

	httpClient := restFake.CreateHTTPClient(func(request *http.Request) (*http.Response, error) {
		require.NotNil(t, request)
		switch request.Method {
		case http.MethodGet:
			{
				assertGet(t, request)
				resp := getVirtSvc(t)
				return resp, nil
			}

		case http.MethodPatch:
			{
				assertPatch(t, request)
				resp := &http.Response{StatusCode: http.StatusNoContent}
				return resp, nil
			}
		default:
			{
				return nil, errors.New(fmt.Sprintf("Not supported method: %s", request.Method))
			}
		}
	})
	//baseURL, err := url.Parse("localhost:8000")
	//require.NoError(t, err)

	restClient := &restFake.RESTClient{
		NegotiatedSerializer: nil,
		GroupVersion:         schema.GroupVersion{},
		VersionedAPIPath:     "",
		Err:                  nil,
		Req:                  nil,
		Client:               httpClient,
		Resp:                 nil,
	}

	//WHEN
	err := p.PatchVirtSvc(restClient, "test", "test-namespace")

	//THEN
	require.NoError(t, err)
}

func getVirtSvc(t *testing.T) *http.Response{
	out, err := ioutil.ReadFile("./test_files/virtual-service.yaml")
	require.NoError(t, err)

	reader := strings.NewReader(string(out))
	body := ioutil.NopCloser(reader)
	resp := &http.Response{Body: body}
	resp.StatusCode = http.StatusOK
	return resp
}

func assertGet(t *testing.T, request *http.Request) {

}
func assertPatch(t *testing.T, request *http.Request) {

}
