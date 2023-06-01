package service

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	NameLabel              = "name"
	SidecarInjectionLabel  = "istio-injection"
	SignifyValidationLabel = "namespaces.warden.kyma-project.io/validate"
)

type NamespaceInterceptor struct {
}

// Intercept adds the name of the namespace also as label to the namespace resource to be backward compatible with Kyma 1.x
func (l *NamespaceInterceptor) Intercept(resources *kubernetes.ResourceCacheList, _ string) error {
	interceptorFunc := func(u *unstructured.Unstructured) error {
		labels := u.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}
		labels[NameLabel] = u.GetName()
		labels[SidecarInjectionLabel] = "enabled"

		//enable Signify signature validation for kyma-system namespace
		if u.GetName() == "kyma-system" {
			labels[SignifyValidationLabel] = "enabled"
		}

		//enable Signify signature validation for istio-system namespace
		if u.GetName() == "istio-system" {
			labels[SignifyValidationLabel] = "enabled"
		}

		u.SetLabels(labels)
		return nil
	}

	return resources.VisitByKind("Namespace", interceptorFunc)
}
