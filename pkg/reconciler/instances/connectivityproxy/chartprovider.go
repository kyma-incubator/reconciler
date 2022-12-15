package connectivityproxy

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	internalKubernetes "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type FilterFunc func([]*unstructured.Unstructured) []*unstructured.Unstructured

func NewProviderWithFilters(provider chart.Provider, filterFuncs ...FilterFunc) chart.Provider {
	return provider.WithFilter(func(manifest string) (string, error) {
		unstructs, err := internalKubernetes.ToUnstructured([]byte(manifest), true)
		if err != nil {
			return "", errors.Wrapf(err, "while casting manifest to kubernetes unstructured")
		}

		for _, filterFunc := range filterFuncs {
			if filterFunc != nil {
				unstructs = filterFunc(unstructs)
			}
		}

		// TODO: serialize unstructs to string
		return manifest, nil
	})
}
