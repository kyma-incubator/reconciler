package service

import (
	"context"
	"strings"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type ServicesInterceptor struct {
	kubeClient kubernetes.Client
}

func (s *ServicesInterceptor) Intercept(resource *unstructured.Unstructured) error {
	if strings.ToLower(resource.GetKind()) != "service" {
		return nil
	}

	svc := &v1.Service{}
	err := runtime.DefaultUnstructuredConverter.
		FromUnstructured(resource.Object, svc)
	if err != nil {
		return err
	}

	if !s.isClusterIPService(svc) {
		return nil
	}

	if svc.Spec.ClusterIP != "" {
		return nil
	}

	clientSet, err := s.kubeClient.Clientset()
	if err != nil {
		return err
	}

	svcResource, err := clientSet.CoreV1().Services(svc.Namespace).Get(context.Background(), svc.Name, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	svc.Spec.ClusterIP = svcResource.Spec.ClusterIP
	unstructObject, err := runtime.DefaultUnstructuredConverter.ToUnstructured(svc)
	if err != nil {
		return err
	}

	resource.Object = unstructObject

	return nil
}

func (s *ServicesInterceptor) isClusterIPService(svc *v1.Service) bool {
	return svc.Spec.Type == v1.ServiceTypeClusterIP || svc.Spec.Type == ""
}
