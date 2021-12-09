package service

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	ManagedByLabel       = "reconciler.kyma-project.io/managed-by"
	KymaVersionLabel     = "reconciler.kyma-project.io/origin-version"
	LabelReconcilerValue = "reconciler"
)

type LabelsInterceptor struct {
	Version string
}

func (l *LabelsInterceptor) Intercept(resources map[string][]*unstructured.Unstructured, _ string) error {
	for kind := range resources {
		for _, resource := range resources[kind] {
			if resource != nil {
				labels := resource.GetLabels()
				if labels == nil {
					labels = make(map[string]string)
				}
				labels[ManagedByLabel] = LabelReconcilerValue
				labels[KymaVersionLabel] = l.Version
				resource.SetLabels(labels)
			}
		}
	}

	return nil
}
