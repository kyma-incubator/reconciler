package service

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"testing"
)

func TestAnnotationsInterceptor(t *testing.T) {
	type args struct {
		resource *unstructured.Unstructured
	}
	tests := []struct {
		name        string
		args        args
		wantErr     bool
		annotations map[string]string
	}{
		{
			name: "Resource without any annotations",
			args: args{
				resource: &unstructured.Unstructured{},
			},
			wantErr: false,
			annotations: map[string]string{
				ManagedByAnnotation: annotationReconcilerValue,
			},
		},
		{
			name: "Resource with annotations",
			args: args{
				resource: &unstructured.Unstructured{
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
			result, err := l.Intercept(tt.args.resource, "")
			require.Equal(t, kubernetes.ContinueInterceptionResult, result)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.Equal(t, tt.annotations, tt.args.resource.GetAnnotations())
			}
		})
	}
}
