package service

import (
	"context"
	k8s "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"reflect"
	"strings"
)

type NoUpdateInterceptor struct {
	kubeClient k8s.Client
	logger     *zap.SugaredLogger
}

func (i *NoUpdateInterceptor) Intercept(resource *unstructured.Unstructured, namespace string) (k8s.InterceptionResult, error) {
	ns := namespace
	if resource.GetNamespace() != "" { //namespace defined in manifest has precedence
		ns = resource.GetNamespace()
	}
	switch strings.ToLower(resource.GetKind()) {
	case "pod":
		return i.checkResourceExistence(resource, ns, func(ctx context.Context, name, namespc string) (interface{}, error) {
			return i.kubeClient.GetPod(ctx, name, namespc)
		})
	case "persistentvolumeclaim":
		return i.checkResourceExistence(resource, ns, func(ctx context.Context, name, namespc string) (interface{}, error) {
			return i.kubeClient.GetPersistentVolumeClaim(ctx, name, namespc)
		})
	}
	return k8s.ContinueInterceptionResult, nil
}

func (i *NoUpdateInterceptor) checkResourceExistence(
	resource *unstructured.Unstructured,
	namespace string,
	lookup func(ctx context.Context, name, namespace string) (interface{}, error)) (k8s.InterceptionResult, error) {
	res, err := lookup(context.Background(), resource.GetName(), namespace)
	if err != nil {
		i.logger.Errorf("Failed to retrieve %s '%s@%s': %s",
			resource.GetKind(), resource.GetName(), resource.GetNamespace(), err)
		return k8s.ErrorInterceptionResult, err //unexpected error occurred
	}
	if i.isNil(res) {
		return k8s.ContinueInterceptionResult, nil //resource does not exist: continue and deploy the resource
	}
	return k8s.IgnoreResourceInterceptionResult, nil //resource exists: ignore it to avoid any updates on the resource
}

//isNil verifies whether the given interface is nil and supports also nil-checks if interface is of kind pointer
func (i *NoUpdateInterceptor) isNil(v interface{}) bool {
	return v == nil || (reflect.ValueOf(v).Kind() == reflect.Ptr && reflect.ValueOf(v).IsNil())
}
