package rendering

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	internalKubernetes "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type FilterFunc func([]*unstructured.Unstructured) ([]*unstructured.Unstructured, error)

func NewProviderWithFilters(provider chart.Provider, filterFuncs ...FilterFunc) chart.Provider {
	return provider.WithFilter(func(manifest string) (string, error) {
		unstructs, err := internalKubernetes.ToUnstructured([]byte(manifest), true)
		if err != nil {
			return "", errors.Wrapf(err, "failed to convert manifest to unstructured object")
		}

		for _, filterFunc := range filterFuncs {
			if filterFunc != nil {
				unstructs, err = filterFunc(unstructs)
				if err != nil {
					return "", err
				}
			}
		}

		newManifest, err := serialize(unstructs)
		if err != nil {
			return "", err
		}

		return newManifest, nil
	})
}

func serialize(unstructs []*unstructured.Unstructured) (string, error) {
	manifests := ""

	for _, unstruct := range unstructs {
		bytes, err := runtime.Encode(unstructured.UnstructuredJSONScheme, unstruct)
		if err != nil {
			return "", errors.Wrapf(err, "failed to encode unstructured object as yaml")
		}

		if manifests != "" {
			manifests += "\n"
		}
		manifests += string(bytes)
		manifests += "---"
	}

	return manifests, nil
}
