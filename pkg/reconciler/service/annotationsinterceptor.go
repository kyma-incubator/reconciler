package service

import (
	k8s "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	ManagedByAnnotation       = "reconciler.kyma-project.io/managed-by-reconciler-disclaimer"
	annotationReconcilerValue = "DO NOT EDIT - This resource is managed by Kyma.\nAny modifications are discarded and the resource is reverted to the original state."
)

type AnnotationsInterceptor struct {
}

func (l *AnnotationsInterceptor) Intercept(resource *unstructured.Unstructured, _ string) (k8s.InterceptionResult, error) {
	annotations := resource.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[ManagedByAnnotation] = annotationReconcilerValue
	resource.SetAnnotations(annotations)

	return k8s.ContinueInterceptionResult, nil
}
