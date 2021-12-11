package service

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func (s *ServicesInterceptor) Intercept(resources *k8s.ResourceList, namespace string) error {
	interceptorFct := func(u *unstructured.Unstructured) error {
		//convert unstruct to service resource
		svc := &v1.Service{}
		err := runtime.DefaultUnstructuredConverter.
			FromUnstructured(u.Object, svc)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to convert unstructured entity '%s@%s' (kind '%s')",
				u.GetName(), u.GetNamespace(), u.GetKind()))
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
		svcInCluster, err := s.kubeClient.GetService(context.Background(), u.GetName(), k8s.ResolveNamespace(u, namespace))
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to get unstructured entity '%s@%s' (kind '%s')",
				u.GetName(), u.GetNamespace(), u.GetKind()))
		}

		//if service exists in cluster, add the missing ClusterIP field using the value already used inside the cluster
		if svcInCluster != nil {
			svc.Spec.ClusterIP = svcInCluster.Spec.ClusterIP //use cluster IP from K8s service resource

			unstructObject, err := runtime.DefaultUnstructuredConverter.ToUnstructured(svc)
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("failed to convert unstructured entity '%s@%s' (kind '%s')",
					u.GetName(), u.GetNamespace(), u.GetKind()))
			}

			u.Object = unstructObject
		}

		return nil
	}

	return resources.VisitByKind("service", interceptorFct)
}

func (s *ServicesInterceptor) isClusterIPService(svc *v1.Service) bool {
	//if spec.Type is undefined, service is treated as ClusterIP service
	return svc.Spec.Type == v1.ServiceTypeClusterIP || svc.Spec.Type == ""
}

func (s *ServicesInterceptor) isNodePortService(svc *v1.Service) bool {
	return svc.Spec.Type == v1.ServiceTypeNodePort
}
