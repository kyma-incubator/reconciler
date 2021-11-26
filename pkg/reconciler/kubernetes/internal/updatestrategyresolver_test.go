package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/rest/fake"
	"k8s.io/kubectl/pkg/scheme"
)

func TestDefaultUpdateStrategyResolver_Resolve(t *testing.T) {

	tests := []struct {
		Name     string
		Response map[string]interface{}
		Resource *unstructured.Unstructured
		Want     UpdateStrategy
		WantErr  bool
	}{
		{
			Name:     "Pods should be skipped",
			Response: nil,
			Resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "Pod",
				},
			},
			Want: SkipUpdateStrategy,
		},
		{
			Name:     "Jobs should be skipped",
			Response: nil,
			Resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "Job",
				},
			},
			Want: SkipUpdateStrategy,
		},
		{
			Name:     "PVCs should be patched",
			Response: nil,
			Resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "PersistentVolumeClaim",
				},
			},
			Want: PatchUpdateStrategy,
		},
		{
			Name:     "ServiceAccounts should be patched",
			Response: nil,
			Resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "ServiceAccount",
				},
			},
			Want: PatchUpdateStrategy,
		},
		{
			Name: "Statefulsets with PVC templates should be patched",
			Response: map[string]interface{}{
				"kind":       "StatefulSet",
				"apiVersion": "apps/v1",
				"spec": map[string]interface{}{
					"volumeClaimTemplates": []map[string]interface{}{
						{
							"kind": "PersistentVolumeClaim",
						},
					},
				},
			},
			Resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "StatefulSet",
					"metadata": map[string]interface{}{
						"name":      "postgresql",
						"namespace": "kyma-system",
					},
				},
			},
			Want: PatchUpdateStrategy,
		},
		{
			Name: "Statefulsets without PVC templates should be replaced",
			Response: map[string]interface{}{
				"kind":       "StatefulSet",
				"apiVersion": "apps/v1",
				"spec":       map[string]interface{}{},
			},
			Resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "StatefulSet",
					"metadata": map[string]interface{}{
						"name":      "postgresql2",
						"namespace": "kyma-system",
					},
				},
			},
			Want: ReplaceUpdateStrategy,
		},
		{
			Name:     "Anything else should be replaces",
			Response: nil,
			Resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "Foooooo",
				},
			},
			Want: ReplaceUpdateStrategy,
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			var helper *resource.Helper
			if tt.Response != nil {

				httpClient := fake.CreateHTTPClient(func(request *http.Request) (*http.Response, error) {

					if request.Method == http.MethodGet {
						return createResponse(t, tt.Response), nil
					}
					return nil, fmt.Errorf("Not supported method: %s", request.Method)
				})

				restClient := &fake.RESTClient{
					// GroupVersion:         appsv1,
					NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
					Client:               httpClient,
				}

				helper = &resource.Helper{
					RESTClient:      restClient,
					Resource:        "StatefulSet",
					NamespaceScoped: true,
				}
			}

			d := newDefaultUpdateStrategyResolver(helper)
			got, err := d.Resolve(tt.Resource)
			if (err != nil) != tt.WantErr {
				t.Errorf("DefaultUpdateStrategyResolver.Resolve() error = %v, wantErr %v", err, tt.WantErr)
				return
			}
			if got != tt.Want {
				t.Errorf("DefaultUpdateStrategyResolver.Resolve() = %v, want %v", got, tt.Want)
			}
		})
	}
}

func createResponse(t *testing.T, responeContent map[string]interface{}) *http.Response {
	o := responeContent
	out, err := json.Marshal(o)
	require.NoError(t, err)
	reader := strings.NewReader(string(out))
	body := io.NopCloser(reader)
	resp := &http.Response{Body: body, StatusCode: http.StatusOK, Header: header()}
	return resp
}

func header() http.Header {
	header := http.Header{}
	header.Set("Content-Type", runtime.ContentTypeJSON)
	return header
}
