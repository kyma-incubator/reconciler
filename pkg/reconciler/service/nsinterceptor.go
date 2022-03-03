package service

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	NameLabel = "name"
)

type NamespaceInterceptor struct {
}

func (l *NamespaceInterceptor) Intercept(resources *kubernetes.ResourceCacheList, _ string) error {
	interceptorFunc := func(u *unstructured.Unstructured) error {
		labels := u.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}
		labels[NameLabel] = u.GetName()
		u.SetLabels(labels)
		return nil
	}

	return resources.VisitByKind("Namespace", interceptorFunc)
}
