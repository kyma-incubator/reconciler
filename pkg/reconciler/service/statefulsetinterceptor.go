package service

import (
	"context"
	"fmt"
	k8s "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"strings"
)

type StatefulSetInterceptor struct {
	kubeClient k8s.Client
	logger     *zap.SugaredLogger
}

func (i *StatefulSetInterceptor) Intercept(resource *unstructured.Unstructured, namespace string) (k8s.InterceptionResult, error) {
	if strings.ToLower(resource.GetKind()) != "statefulset" {
		return k8s.ContinueInterceptionResult, nil
	}

	ns := namespace
	if resource.GetNamespace() != "" {
		ns = resource.GetNamespace()
	}

	sfs, err := i.kubeClient.GetStatefulSet(context.Background(), resource.GetName(), ns)
	if err != nil {
		return k8s.ErrorInterceptionResult, err
	}

	if sfs != nil && len(sfs.Spec.VolumeClaimTemplates) > 0 { //do not replace STS which have a PVC inside
		return k8s.IgnoreResourceInterceptionResult, nil
	}

	return k8s.ContinueInterceptionResult, nil
}

func (i *StatefulSetInterceptor) specFieldMissing(resource *unstructured.Unstructured) (k8s.InterceptionResult, error) {
	errMsg := "given statefulSet doesn't include a 'spec' field"
	resourceYaml, err := yaml.Marshal(resource.Object)
	if err != nil {
		return k8s.ErrorInterceptionResult, fmt.Errorf("%s and serialization to YAML failed: %s", errMsg, err)
	}
	return k8s.ErrorInterceptionResult, fmt.Errorf("%s: %s", errMsg, resourceYaml)
}
