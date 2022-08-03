package service

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	NameLabel             = "name"
	SidecarInjectionLabel = "istio-injection"
)

type NamespaceInterceptor struct {
}

//Intercept adds the name of the namespace also as label to the namespace resource to be backward compatible with Kyma 1.x
func (l *NamespaceInterceptor) Intercept(resources *kubernetes.ResourceCacheList, _ string) error {
	interceptorFunc := func(u *unstructured.Unstructured) error {
		labels := u.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}
		labels[NameLabel] = u.GetName()
		labels[SidecarInjectionLabel] = "enabled"
		u.SetLabels(labels)
		return nil
	}

	return resources.VisitByKind("Namespace", interceptorFunc)
}
