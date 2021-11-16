package service

import (
	"context"
	"strings"

	k8s "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type ServicesInterceptor struct {
	kubeClient k8s.Client
}

func (s *ServicesInterceptor) Intercept(resource *unstructured.Unstructured, namespace string) (k8s.InterceptionResult, error) {
	if strings.ToLower(resource.GetKind()) != "service" {
		return k8s.ContinueInterceptionResult, nil
	}

	svc := &v1.Service{}
	err := runtime.DefaultUnstructuredConverter.
		FromUnstructured(resource.Object, svc)
	if err != nil {
		return k8s.ErrorInterceptionResult, err
	}

	if !s.isClusterIPService(svc) || svc.Spec.ClusterIP != "" { //not a clusterIP service or clusterIP-field is defined
		return k8s.ContinueInterceptionResult, nil
	}

	svcInCluster, err := s.kubeClient.GetService(context.Background(), resource.GetName(), namespace)
	if err != nil {
		return k8s.ErrorInterceptionResult, err
	}

	if svcInCluster != nil {
		svc.Spec.ClusterIP = svcInCluster.Spec.ClusterIP //use cluster IP from K8s service resource

		unstructObject, err := runtime.DefaultUnstructuredConverter.ToUnstructured(svc)
		if err != nil {
			return k8s.ErrorInterceptionResult, err
		}

		resource.Object = unstructObject
	}

	return k8s.ContinueInterceptionResult, nil
}

func (s *ServicesInterceptor) isClusterIPService(svc *v1.Service) bool {
	return svc.Spec.Type == v1.ServiceTypeClusterIP || svc.Spec.Type == ""
}
