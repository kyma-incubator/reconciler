package service

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	ManagedByAnnotation       = "reconciler.kyma-project.io/managed-by-reconciler-disclaimer"
	AnnotationReconcilerValue = "This resource is managed [by SAP]. Don't make any changes. Changes made by users are reverted automatically."
)

type AnnotationsInterceptor struct {
}

func (l *AnnotationsInterceptor) Intercept(resource *unstructured.Unstructured) error {
	annotations := resource.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[ManagedByAnnotation] = AnnotationReconcilerValue
	resource.SetAnnotations(annotations)
	return nil
}
