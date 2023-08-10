package get

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"reflect"
	"testing"
)

func TestGetIstioCRManifest(t *testing.T) {
	type args struct {
		url string
		tag string
	}
	tests := []struct {
		name         string
		args         args
		wantManifest unstructured.Unstructured
		wantErr      bool
	}{
		{
			name: "Check if fetching release 1.0.0 return correct Istio CR",
			args: args{
				url: "https://github.com/kyma-project/istio/releases/download",
				tag: "1.0.0",
			},
			wantManifest: unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "operator.kyma-project.io/v1alpha2",
				"kind":       "Istio",
				"metadata": map[string]interface{}{
					"name":      "default",
					"namespace": "kyma-system",
					"labels": map[string]interface{}{
						"app.kubernetes.io/name": "default",
					},
				},
			}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotManifest, err := IstioCRManifest(tt.args.url, tt.args.tag)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetIstioCRManifest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotManifest, tt.wantManifest) {
				t.Errorf("GetIstioCRManifest() gotManifest = %v, want %v", gotManifest, tt.wantManifest)
			}
		})
	}
}

func TestGetIstioManagerManifest(t *testing.T) {
	type args struct {
		url string
		tag string
	}
	tests := []struct {
		name    string
		args    args
		wantLen int
		wantErr bool
	}{
		{
			name: "Check if fetching release 1.0.0 returns 10 manifests",
			args: args{
				url: "https://github.com/kyma-project/istio/releases/download",
				tag: "1.0.0",
			},
			wantLen: 10,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IstioManagerManifest(tt.args.url, tt.args.tag)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetIstioManagerManifest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != tt.wantLen {
				t.Errorf("GetIstioManagerManifest() got lenght = %v, want length %v", len(got), tt.wantLen)
			}
		})
	}
}
