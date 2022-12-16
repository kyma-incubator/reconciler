package rendering

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	ReleaseLabelKey       = "release"
	ConnectivityProxyKind = "StatefulSet"
)

func NewFilterByAnnotation(annotation string) FilterFunc {
	return func(unstructs []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
		newUnstructs := make([]*unstructured.Unstructured, 0, 0)

		for _, unstruct := range unstructs {
			annotations := unstruct.GetAnnotations()
			_, ok := annotations[annotation]
			if !ok {
				newUnstructs = append(newUnstructs, unstruct)
			}
		}

		return newUnstructs, nil
	}
}

func NewFilterByRelease(context *service.ActionContext, appName, appRelease string) FilterFunc {
	return func(unstructs []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
		var statefulSetManifest *unstructured.Unstructured
		for _, unstruct := range unstructs {
			if unstruct != nil && unstruct.GetName() == appName && unstruct.GetKind() == ConnectivityProxyKind {
				statefulSetManifest = unstruct
				break
			}
		}

		if statefulSetManifest == nil {
			context.Logger.Warn("Did not find the Connectivity Proxy stateful set, skipping")
			return nil, errors.Errorf("Connectivity Proxy stateful set does not have any release labels")
		}

		if statefulSetManifest.GetLabels() == nil || statefulSetManifest.GetLabels()[ReleaseLabelKey] == "" {
			return nil, errors.Errorf("Connectivity Proxy StatefulSet does not have any release labels")
		}

		if statefulSetManifest.GetLabels()[ReleaseLabelKey] != appRelease {
			context.Logger.Debug("Connectivity Proxy release has changed, the component will be upgraded")
			return unstructs, nil
		}

		context.Logger.Debug("Connectivity Proxy release did not change, skipping")
		return nil, nil
	}
}
