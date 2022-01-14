package service

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestAnnotationsInterceptor(t *testing.T) {
	tests := []struct {
		name        string
		unstruct    *unstructured.Unstructured
		wantErr     bool
		annotations map[string]string
	}{
		{
			name:     "Resource without any annotations",
			unstruct: &unstructured.Unstructured{},
			wantErr:  false,
			annotations: map[string]string{
				ManagedByAnnotation: annotationReconcilerValue,
			},
		},
		{
			name: "Resource with annotations",
			unstruct: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"some-annotation":  "some-value",
							"some-annotation2": "some-value2",
						},
					},
				},
			},
			wantErr: false,
			annotations: map[string]string{
				"some-annotation":   "some-value",
				"some-annotation2":  "some-value2",
				ManagedByAnnotation: annotationReconcilerValue,
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			l := &AnnotationsInterceptor{}

			resources := kubernetes.NewResourceList([]*unstructured.Unstructured{
				tt.unstruct,
			})

			err := l.Intercept(resources, "")
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.Equal(t, tt.annotations, tt.unstruct.GetAnnotations())
			}
		})
	}
}
