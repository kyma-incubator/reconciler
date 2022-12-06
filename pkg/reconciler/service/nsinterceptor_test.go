package service

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNamespaceInterceptor(t *testing.T) {
	tests := []struct {
		name           string
		resource       *unstructured.Unstructured
		expectedLabels map[string]string
	}{
		{
			name: "Namespace with labels",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Namespace",
					"metadata": map[string]interface{}{
						"name": "namespace1",
						"labels": map[string]interface{}{
							"some-label": "some-value",
						},
					},
				},
			},
			expectedLabels: map[string]string{
				SidecarInjectionLabel: "enabled",
				"some-label":          "some-value",
				NameLabel:             "namespace1",
			},
		},
		{
			name: "Namespace without labels",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Namespace",
					"metadata": map[string]interface{}{
						"name": "namespace2",
					},
				},
			},
			expectedLabels: map[string]string{
				SidecarInjectionLabel: "enabled",
				NameLabel:             "namespace2",
			},
		},
		{
			name: "Namespace kyma-system with labels",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Namespace",
					"metadata": map[string]interface{}{
						"name": "kyma-system",
						"labels": map[string]interface{}{
							"some-label": "some-value",
						},
					},
				},
			},
			expectedLabels: map[string]string{
				SidecarInjectionLabel:  "enabled",
				"some-label":           "some-value",
				NameLabel:              "kyma-system",
				SignifyValidationLabel: "enabled",
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			l := &NamespaceInterceptor{}
			resources := kubernetes.NewResourceList([]*unstructured.Unstructured{tt.resource})
			err := l.Intercept(resources, "")
			require.NoError(t, err)
			require.Equal(t, tt.expectedLabels, tt.resource.GetLabels())
		})
	}
}
