package service

import (
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestClusterWideResourceInterceptor_Intercept(t *testing.T) {

	tests := []struct {
		name              string
		resource          *unstructured.Unstructured
		wantErr           bool
		expectedNamespace string
	}{
		{
			name: "Should not clear namespace for deployments",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"namespace": "foo",
					},
				},
			},
			wantErr:           false,
			expectedNamespace: "foo",
		},
		{
			name: "Should clear namespace for rbac.authorization.k8s.io/v1/ClusterRole",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "rbac.authorization.k8s.io/v1",
					"kind":       "ClusterRole",
					"metadata": map[string]interface{}{
						"namespace": "foo",
					},
				},
			},
			wantErr:           false,
			expectedNamespace: "",
		},
		{
			name: "Should not clear namespace for yada/ClusterRole",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "yada",
					"kind":       "ClusterRole",
					"metadata": map[string]interface{}{
						"namespace": "foo",
					},
				},
			},
			wantErr:           false,
			expectedNamespace: "foo",
		},
	}
	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			i := newClusterWideResourceInterceptor()
			if err := i.Intercept(kubernetes.NewResourceList([]*unstructured.Unstructured{testCase.resource}), ""); (err != nil) != testCase.wantErr {
				t.Errorf("ClusterWideResourceInterceptor.Intercept() error = %v, wantErr %v", err, testCase.wantErr)
			}
			require.Equal(t, testCase.expectedNamespace, testCase.resource.GetNamespace())
		})
	}
}
