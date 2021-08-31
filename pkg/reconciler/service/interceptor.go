package service

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type LabelInterceptor struct {
}

func (l *LabelInterceptor) Intercept(resource *unstructured.Unstructured) error {
	labels := resource.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[reconciler.ManagedByLabel] = reconciler.LabelReconcilerValue
	resource.SetLabels(labels)
	return nil
}
