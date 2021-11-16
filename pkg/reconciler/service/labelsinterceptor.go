package service

import (
	k8s "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
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

func (l *LabelsInterceptor) Intercept(resource *unstructured.Unstructured, _ string) (k8s.InterceptionResult, error) {
	labels := resource.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[ManagedByLabel] = LabelReconcilerValue
	labels[KymaVersionLabel] = l.Version
	resource.SetLabels(labels)

	return k8s.ContinueInterceptionResult, nil
}
