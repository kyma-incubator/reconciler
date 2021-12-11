package service

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	ManagedByAnnotation       = "reconciler.kyma-project.io/managed-by-reconciler-disclaimer"
	annotationReconcilerValue = "DO NOT EDIT - This resource is managed by Kyma.\nAny modifications are discarded and the resource is reverted to the original state."
)

type AnnotationsInterceptor struct {
}

func (l *AnnotationsInterceptor) Intercept(resources *kubernetes.ResourceList, _ string) error {
	interceptorFunc := func(u *unstructured.Unstructured) error {
		annotations := u.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[ManagedByAnnotation] = annotationReconcilerValue
		u.SetAnnotations(annotations)
		return nil
	}

	return resources.Visit(interceptorFunc)
}
