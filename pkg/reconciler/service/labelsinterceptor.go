package service

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
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

func (l *LabelsInterceptor) Intercept(resources kubernetes.Resources, _ string) error {
	interceptorFunc := func(kind string, u *unstructured.Unstructured) error {
		labels := u.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}
		labels[ManagedByLabel] = LabelReconcilerValue
		labels[KymaVersionLabel] = l.Version
		u.SetLabels(labels)
		return nil
	}

	return resources.Visit(interceptorFunc)
}
