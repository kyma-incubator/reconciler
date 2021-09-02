package service

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	ManagedByLabel       = "reconciler.kyma-project.io/managed-by"
	LabelReconcilerValue = "reconciler"
)

type LabelInterceptor struct {
}

func (l *LabelInterceptor) Intercept(resource *unstructured.Unstructured) error {
	labels := resource.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[ManagedByLabel] = LabelReconcilerValue
	resource.SetLabels(labels)
	return nil
}
