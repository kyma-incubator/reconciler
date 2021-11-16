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

	//resolve namespace
	ns := namespace
	if resource.GetNamespace() != "" { //namespace defined in manifest has precedence
		ns = resource.GetNamespace()
	}

	//convert unstruct to service resource
	svc := &v1.Service{}
	err := runtime.DefaultUnstructuredConverter.
		FromUnstructured(resource.Object, svc)
	if err != nil {
		return k8s.ErrorInterceptionResult, err
	}

	//verify whether the service is of type IPCluster or NodePortService
	if !(s.isClusterIPService(svc) || s.isNodePortService(svc)) {
		return k8s.ContinueInterceptionResult, nil
	}

	//adjust the ClusterIP field only if it is empty
	if svc.Spec.ClusterIP != "" {
		return k8s.ContinueInterceptionResult, nil
	}

	//retrieve existing service from cluster
	svcInCluster, err := s.kubeClient.GetService(context.Background(), resource.GetName(), ns)
	if err != nil {
		return k8s.ErrorInterceptionResult, err
	}

	//if service exists in cluster, add the missing ClusterIP field using the value already used inside the cluster
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
	//if spec.Type is undefined, service is treated as ClusterIP service
	return svc.Spec.Type == v1.ServiceTypeClusterIP || svc.Spec.Type == ""
}

func (s *ServicesInterceptor) isNodePortService(svc *v1.Service) bool {
	return svc.Spec.Type == v1.ServiceTypeNodePort
}
