package service

import (
	"fmt"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"testing"
)

func TestLabelInterceptor(t *testing.T) {
	type args struct {
		resource *unstructured.Unstructured
		version  string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		labels  map[string]string
	}{
		{
			name: "Resource without any labels",
			args: args{
				resource: &unstructured.Unstructured{},
				version:  "1.19.0",
			},
			wantErr: false,
			labels: map[string]string{
				ManagedByLabel:   LabelReconcilerValue,
				KymaVersionLabel: "1.19.0",
			},
		},
		{
			name: "Resource with labels",
			args: args{
				resource: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"labels": map[string]interface{}{
								"some-label":  "some-value",
								"some-label2": "some-value2",
							},
						},
					},
				},
				version: "1.19.0",
			},
			wantErr: false,
			labels: map[string]string{
				"some-label":     "some-value",
				"some-label2":    "some-value2",
				ManagedByLabel:   LabelReconcilerValue,
				KymaVersionLabel: "1.19.0",
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			l := &LabelsInterceptor{Version: tt.args.version}
			if err := l.Intercept(tt.args.resource); (err != nil) != tt.wantErr {
				t.Errorf("Intercept() error = %v, wantErr %v", err, tt.wantErr)
			}
			if fmt.Sprint(tt.labels) != fmt.Sprint(tt.args.resource.GetLabels()) {
				t.Errorf("Actual labels: %s aren't the same like expected labels: %s", tt.args.resource.GetLabels(), tt.labels)
			}
		})
	}
}
