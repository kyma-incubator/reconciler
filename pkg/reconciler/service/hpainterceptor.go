package service

import (
	"context"
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	autoscalingv2 "k8s.io/api/autoscaling/v2beta2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type HPAInterceptor struct {
	kubeClient kubernetes.Client
	logger     *zap.SugaredLogger
}

func (i *HPAInterceptor) Intercept(resources *kubernetes.ResourceList, namespace string) error {
	interceptorFunc := func(u *unstructured.Unstructured) error {
		//convert unstruct to HPA resource
		hpa := &autoscalingv2.HorizontalPodAutoscaler{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, hpa); err != nil {
			return errors.Wrap(err, fmt.Sprintf("hpa interceptor failed to convert unstructured entity '%s'",
				u.GetName()))
		}

		maxReplicas := hpa.Spec.MaxReplicas
		minReplicas := *hpa.Spec.MinReplicas

		referencedResource := resources.Get(hpa.Spec.ScaleTargetRef.Kind, hpa.Spec.ScaleTargetRef.Name, namespace)
		if referencedResource == nil {
			i.logger.Warnf("Could not find the referenced resource '%s/%s' (namespace: %s) by the HPA '%s'", hpa.Spec.ScaleTargetRef.Kind, hpa.Spec.ScaleTargetRef.Name, namespace, hpa.GetName())
			return nil
		}

		if referencedResource.GetKind() == "Deployment" {
			deployment, err := i.kubeClient.GetDeployment(context.Background(), referencedResource.GetName(), referencedResource.GetNamespace())
			if err != nil {
				return err
			}

			if deployment == nil {
				if *deployment.Spec.Replicas < minReplicas || *deployment.Spec.Replicas > maxReplicas {
					i.logger.Warnf("Replica field of the deployment '%s' (namespace: %s) is out of range of the configured HPA maxValue / minValue", deployment.GetName(), deployment.GetNamespace())
					return nil
				}
			}

			if err := unstructured.SetNestedField(referencedResource.Object, deployment.Spec.Replicas, "spec", "replicas"); err != nil {
				i.logger.Errorf("Failed to set replica count of the deployment '%s' (namespace: %s) referenced by an HPA: %s", referencedResource.GetName(), referencedResource.GetNamespace(), err)
			}

		} else if referencedResource.GetKind() == "StatefulSet" {
			ss, err := i.kubeClient.GetStatefulSet(context.Background(), referencedResource.GetName(), referencedResource.GetNamespace())
			if err != nil {
				return err
			}

			if ss == nil {
				if *ss.Spec.Replicas < minReplicas || *ss.Spec.Replicas > maxReplicas {
					i.logger.Warnf("Replica field of the statefulset '%s' (namespace: %s) is out of range of the configured HPA maxValue / minValue", ss.GetName(), ss.GetNamespace())
					return nil
				}
			}

			if err := unstructured.SetNestedField(referencedResource.Object, ss.Spec.Replicas, "spec", "replicas"); err != nil {
				i.logger.Errorf("Failed to set replica count of the statefulset '%s' (namespace: %s) referenced by an HPA: %s", referencedResource.GetName(), referencedResource.GetNamespace(), err)
			}
		} else {
			i.logger.Warnf("Unsupported kind for HPA Interceptor: %s", referencedResource.GetKind())
		}

		return nil
	}

	return resources.VisitByKind("HorizontalPodAutoscaler", interceptorFunc)
}
