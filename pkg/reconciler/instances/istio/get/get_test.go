package get

import (
	_ "embed"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

const sampleIstioConfiguration = `
kind: IstioOperatorConfiguration
tag: "1.0.0"

`

//go:embed istio-default-cr.yaml
var sampleIstioCR []byte

//go:embed istio-manager.yaml
var sampleManifest []byte

func TestGetIstioCRManifest(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/kyma-project/istio/releases/download/1.0.0/istio-default-cr.yaml", handleCR)
	s := httptest.NewServer(mux)
	defer s.Close()

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
				url: s.URL + "/kyma-project/istio/releases/download",
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

func handleManifest(w http.ResponseWriter, r *http.Request) {
	_, err := w.Write(sampleManifest)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func handleCR(w http.ResponseWriter, r *http.Request) {
	_, err := w.Write(sampleIstioCR)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func TestGetIstioManagerManifest(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/kyma-project/istio/releases/download/1.0.0/istio-manager.yaml", handleManifest)
	s := httptest.NewServer(mux)
	defer s.Close()

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
				url: s.URL + "/kyma-project/istio/releases/download",
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

func TestIstioTagFromContext(t *testing.T) {
	type args struct {
		context *service.ActionContext
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "Should get 1.0.0 from sampleIstioConfiguration",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IstioTagFromContext(tt.args.context)
			if (err != nil) != tt.wantErr {
				t.Errorf("IstioTagFromContext() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IstioTagFromContext() got = %v, want %v", got, tt.want)
			}
		})
	}
}
