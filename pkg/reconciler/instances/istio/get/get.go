package get

import (
	"bytes"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	yaml3 "gopkg.in/yaml.v3"
	"io"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"net/http"
)

// The kind is used for easy access to the configuration in Kyma chart
const configurationKind = "IstioOperatorConfiguration"

func IstioTagFromContext(context *service.ActionContext) (string, error) {
	component := chart.NewComponentBuilder(context.Task.Version, context.Task.Component).Build()
	istioManifest, err := context.ChartProvider.RenderManifest(component)
	if err != nil {
		return "", err
	}

	man, err := kubernetes.ToUnstructured([]byte(istioManifest.Manifest), true)
	if err != nil {
		return "", err
	}

	var istioVersion string
	for _, u := range man {
		if u.GetKind() == configurationKind {
			version, ok := u.Object["tag"]
			if ok {
				istioVersion = version.(string)
			} else {
				return "", errors.New("Tag wasn't present in chart")
			}
		}
	}
	if istioVersion == "" {
		return "", errors.New("Didn't find istio operator configuration in chart")
	}

	return istioVersion, nil
}

func IstioManagerManifest(url, tag string) ([]unstructured.Unstructured, error) {
	resp, err := http.Get(fmt.Sprintf("%s/%s/istio-manager.yaml", url, tag))
	if err != nil {
		return nil, err
	}

	rawManifests, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var manifests []unstructured.Unstructured
	decoder := yaml3.NewDecoder(bytes.NewBuffer(rawManifests))
	for {
		var d map[string]interface{}
		if err := decoder.Decode(&d); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("Document decode failed: %w", err)
		}
		manifests = append(manifests, unstructured.Unstructured{Object: d})
	}

	return manifests, nil
}

func IstioCRManifest(url, tag string) (unstructured.Unstructured, error) {
	resp, err := http.Get(fmt.Sprintf("%s/%s/istio-default-cr.yaml", url, tag))
	if err != nil {
		return unstructured.Unstructured{}, err
	}
	rawManifest, err := io.ReadAll(resp.Body)
	if err != nil {
		return unstructured.Unstructured{}, err
	}
	var manifest unstructured.Unstructured
	err = yaml3.Unmarshal(rawManifest, &manifest.Object)
	return manifest, err
}
