package service

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	k8serr "k8s.io/apimachinery/pkg/api/errors"
)

type FinalizerInterceptor struct {
	kubeClient         kubernetes.Client
	interceptableKinds []string
}

// Intercept preserves finalizers on all kinds in interceptableKinds or all found resources if not specified
func (i *FinalizerInterceptor) Intercept(resources *kubernetes.ResourceCacheList, _ string) error {
	interceptorFunc := func(u *unstructured.Unstructured) error {
		existingResource, err := i.kubeClient.Get(u.GetKind(), u.GetName(), u.GetNamespace())
		if err != nil {
			// Object is newly created, no finalizers to preserve
			if k8serr.IsNotFound(err) {
				return nil
			}
			return err
		}

		if existingResource.GetFinalizers() != nil {
			u.SetFinalizers(existingResource.GetFinalizers())
		}
		return nil
	}

	// Apply to all kinds if not defined
	if i.interceptableKinds == nil {
		err := resources.Visit(interceptorFunc)
		if err != nil {
			return err
		}
	}

	for j := range i.interceptableKinds {
		err := resources.VisitByKind(i.interceptableKinds[j], interceptorFunc)
		if err != nil {
			return err
		}
	}

	return nil
}
