package rendering

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	internalKubernetes "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
)

type FilterFunc func([]*unstructured.Unstructured) ([]*unstructured.Unstructured, error)

func NewProviderWithFilters(provider chart.Provider, filterFuncs ...FilterFunc) chart.Provider {
	return provider.WithFilter(func(manifest string) (string, error) {
		if len(filterFuncs) == 0 {
			return manifest, nil
		}

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

	for index, unstruct := range unstructs {
		serializer := jsonserializer.NewSerializerWithOptions(
			jsonserializer.DefaultMetaFactory,
			scheme.Scheme,
			scheme.Scheme,
			jsonserializer.SerializerOptions{
				Yaml:   true,
				Pretty: false,
				Strict: false,
			},
		)
		yaml, err := runtime.Encode(serializer, unstruct)
		if err != nil {
			return "", errors.Wrapf(err, "failed to encode unstructured object as yaml")
		}

		if index > 0 {
			manifests += "\n"
		}

		manifests += string(yaml)

		if index < len(unstructs)-1 {
			manifests += "---"
		}
	}

	return manifests, nil
}
