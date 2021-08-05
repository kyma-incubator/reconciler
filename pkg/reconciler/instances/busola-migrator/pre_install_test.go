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
	restFake "k8s.io/client-go/rest/fake"
	"net/http"
	"path"
	"strings"
	"testing"
)

func TestNewVirtualServicePreInstallPatch(t *testing.T) {
	//GIVEN
	name := "test"
	namespace := "test-namespace"
	vs := []VirtualSvcMeta{{Name: name, Namespace: namespace}}
	prefix := "-old"
	expectedCRPath := path.Join("/apis", "networking.istio.io/v1alpha3", "namespaces", namespace, "virtualservices", name)

	testCases := []struct {
		Name           string
		ExpectedPatch  virtualServicePatch
		GetVirtSvcFn   func(t *testing.T) *http.Response
		PatchVirtSvcFn func(t *testing.T) *http.Response
		ExpectedError  error
	}{
		{
			Name:           "Success",
			ExpectedPatch:  virtualServicePatch{Spec: specPatch{Hosts: []string{"my-domain-old.kyma.io"}}},
			GetVirtSvcFn:   createResponseFromTestFile,
			PatchVirtSvcFn: createPatchResponse,
			ExpectedError:  nil,
		},
		{
			Name:          "Patching of virtual service failed",
			ExpectedPatch: virtualServicePatch{Spec: specPatch{Hosts: []string{"my-domain-old.kyma.io"}}},
			GetVirtSvcFn:  createResponseFromTestFile,
			PatchVirtSvcFn: func(t *testing.T) *http.Response {
				return &http.Response{StatusCode: http.StatusForbidden}
			},
			ExpectedError: errors.New("while patching virtual service"),
		},
		{
			Name: "Getting of virtual service failed",
			GetVirtSvcFn: func(t *testing.T) *http.Response {
				return &http.Response{StatusCode: http.StatusForbidden}
			},
			PatchVirtSvcFn: createPatchResponse,
			ExpectedError:  errors.New("while getting virtual service"),
		},
		{
			Name:           "Host name is `test`",
			GetVirtSvcFn:   getCreateMinimalResponseFn("test"),
			ExpectedPatch:  virtualServicePatch{Spec: specPatch{Hosts: []string{"test-old"}}},
			PatchVirtSvcFn: createPatchResponse,
			ExpectedError:  nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			p := NewVirtualServicePreInstallPatch(vs, prefix)
			httpClient := restFake.CreateHTTPClient(func(request *http.Request) (*http.Response, error) {
				require.NotNil(t, request)
				switch request.Method {
				case http.MethodGet:
					{
						assertGet(t, request, expectedCRPath)
						resp := testCase.GetVirtSvcFn(t)
						return resp, nil
					}
				case http.MethodPatch:
					{
						assertPatch(t, request, testCase.ExpectedPatch)
						resp := testCase.PatchVirtSvcFn(t)
						return resp, nil
					}
				default:
					{
						return nil, errors.New(fmt.Sprintf("Not supported method: %s", request.Method))
					}
				}
			})

			restClient := &restFake.RESTClient{Client: httpClient}

			ctx := context.TODO()

			//WHEN
			err := p.patchVirtSvc(ctx, restClient, name, namespace)

			//THEN
			if testCase.ExpectedError == nil {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), testCase.ExpectedError.Error())
			}
		})
	}

}

func createResponseFromTestFile(t *testing.T) *http.Response {
	out, err := ioutil.ReadFile("./test_files/virtual-service.yaml")
	require.NoError(t, err)
	reader := strings.NewReader(string(out))
	body := io.NopCloser(reader)
	resp := &http.Response{Body: body}
	resp.StatusCode = http.StatusOK
	return resp
}

func getCreateMinimalResponseFn(host string) func(t *testing.T) *http.Response {
	return func(t *testing.T) *http.Response {
		virtSvc := virtSvc{Spec: virtSvcSpec{Hosts: []string{host}}}
		out, err := json.Marshal(&virtSvc)
		require.NoError(t, err)
		reader := strings.NewReader(string(out))
		body := io.NopCloser(reader)
		resp := &http.Response{Body: body}
		resp.StatusCode = http.StatusOK
		return resp
	}
}

func createPatchResponse(t *testing.T) *http.Response {
	resp := &http.Response{StatusCode: http.StatusNoContent}
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
