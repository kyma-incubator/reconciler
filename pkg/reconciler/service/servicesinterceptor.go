package service

import (
	"context"
	"fmt"
	"strings"

	k8s "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	none = "None"
)

type ServicesInterceptor struct {
	kubeClient k8s.Client
}

func (s *ServicesInterceptor) Intercept(resources k8s.Resources, namespace string) error {
	serviceResources := resources.Get("service")
	if serviceResources == nil {
		return nil
	}

	for _, resource := range serviceResources {
		if resource != nil {
			//convert unstruct to service resource
			svc := &v1.Service{}
			err := runtime.DefaultUnstructuredConverter.
				FromUnstructured(resource.Object, svc)
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("failed to convert unstructured entity '%s@%s' (kind '%s')",
					resource.GetName(), resource.GetNamespace(), resource.GetKind()))
			}

			//verify whether the service is of type IPCluster or NodePortService
			if !(s.isClusterIPService(svc) || s.isNodePortService(svc)) {
				return nil
			}

			//adjust the ClusterIP field only if it is empty or equals to "None"
			if svc.Spec.ClusterIP != "" && !strings.EqualFold(svc.Spec.ClusterIP, none) {
				return nil
			}

			//retrieve existing service from cluster
			svcInCluster, err := s.kubeClient.GetService(context.Background(), resource.GetName(), k8s.ResolveNamespace(resource, namespace))
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("failed to get unstructured entity '%s@%s' (kind '%s')",
					resource.GetName(), resource.GetNamespace(), resource.GetKind()))
			}

			//if service exists in cluster, add the missing ClusterIP field using the value already used inside the cluster
			if svcInCluster != nil {
				svc.Spec.ClusterIP = svcInCluster.Spec.ClusterIP //use cluster IP from K8s service resource

				unstructObject, err := runtime.DefaultUnstructuredConverter.ToUnstructured(svc)
				if err != nil {
					return errors.Wrap(err, fmt.Sprintf("failed to convert unstructured entity '%s@%s' (kind '%s')",
						resource.GetName(), resource.GetNamespace(), resource.GetKind()))
				}

				resource.Object = unstructObject
			}
		}
	}

	return nil
}

func (s *ServicesInterceptor) isClusterIPService(svc *v1.Service) bool {
	//if spec.Type is undefined, service is treated as ClusterIP service
	return svc.Spec.Type == v1.ServiceTypeClusterIP || svc.Spec.Type == ""
}

func (s *ServicesInterceptor) isNodePortService(svc *v1.Service) bool {
	return svc.Spec.Type == v1.ServiceTypeNodePort
}
