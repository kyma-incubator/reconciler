package service

import (
	"fmt"
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
			if err := l.Intercept(tt.args.resource); (err != nil) != tt.wantErr {
				t.Errorf("Intercept() error = %v, wantErr %v", err, tt.wantErr)
			}
			if fmt.Sprint(tt.annotations) != fmt.Sprint(tt.args.resource.GetAnnotations()) {
				t.Errorf("Actual annotations: %s aren't the same like expected annotations: %s", tt.args.resource.GetAnnotations(), tt.annotations)
			}
		})
	}
}
