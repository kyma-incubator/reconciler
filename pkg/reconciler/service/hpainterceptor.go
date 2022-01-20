package service

import (
	"context"
	"fmt"
	v1 "k8s.io/api/apps/v1"

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

func (i *HPAInterceptor) Intercept(resources *kubernetes.ResourceCacheList, namespace string) error {
	interceptorFunc := func(u *unstructured.Unstructured) error {
		namespace := kubernetes.ResolveNamespace(u, namespace)

		//convert unstruct to HPA resource
		hpa := &autoscalingv2.HorizontalPodAutoscaler{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, hpa); err != nil {
			return errors.Wrap(err, fmt.Sprintf("hpa interceptor failed to convert unstructured entity '%s'",
				u.GetName()))
		}

		maxReplicas := hpa.Spec.MaxReplicas
		minReplicas := *hpa.Spec.MinReplicas

		refResource := resources.Get(hpa.Spec.ScaleTargetRef.Kind, hpa.Spec.ScaleTargetRef.Name, namespace)
		if refResource == nil {
			i.logger.Warnf("Could not find the referenced resource '%s/%s' (namespace: %s) by the HPA '%s'",
				hpa.Spec.ScaleTargetRef.Kind, hpa.Spec.ScaleTargetRef.Name, namespace, hpa.GetName())
			return nil
		}

		refNamespace := kubernetes.ResolveNamespace(refResource, namespace)
		if refResource.GetKind() == "Deployment" {
			return i.interceptDeployment(refResource, refNamespace, minReplicas, maxReplicas)
		} else if refResource.GetKind() == "StatefulSet" {
			return i.interceptStatefulSet(refResource, refNamespace, minReplicas, maxReplicas)
		} else {
			i.logger.Warnf("Unsupported kind for HPA Interceptor: %s", refResource.GetKind())
		}

		return nil
	}

	return resources.VisitByKind("HorizontalPodAutoscaler", interceptorFunc)
}

func (i *HPAInterceptor) interceptDeployment(refResource *unstructured.Unstructured, refNamespace string, minReplicas int32, maxReplicas int32) error {
	deployment, err := i.kubeClient.GetDeployment(context.Background(), refResource.GetName(), refNamespace)
	if err != nil {
		return err
	}

	//deployment does not exist on cluster, verify replicaset defined in manifest
	if deployment == nil {
		deployment, err := i.toDeployment(refResource)
		if err != nil {
			return errors.Wrap(err, "hpa interceptor failed to convert unstructured to deployment")
		}
		if i.isReplicaInRange(deployment.Spec.Replicas, minReplicas, maxReplicas) {
			i.logger.Warnf("HPA reconciler detected manifest inconsistency: replicas of deployment '%s@%s' "+
				"is %d but range of replicas in HPA is %d-%d (min-max)",
				deployment.GetName(), refNamespace, *deployment.Spec.Replicas, minReplicas, maxReplicas)
		}
		return nil
	}

	//deployment exists in cluster: update replicas field in manifest
	err = unstructured.SetNestedField(refResource.Object, deployment.Spec.Replicas, "spec", "replicas")
	if err != nil {
		i.logger.Errorf("Failed to set replica count of the deployment '%s' (namespace: %s) "+
			"referenced by an HPA: %s", refResource.GetName(), refNamespace, err)
	}
	return err
}

func (i *HPAInterceptor) interceptStatefulSet(refResource *unstructured.Unstructured, refNamespace string, minReplicas int32, maxReplicas int32) error {
	sfs, err := i.kubeClient.GetStatefulSet(context.Background(), refResource.GetName(), refNamespace)
	if err != nil {
		return err
	}

	//statefuleset does not exist on cluster, verify replicaset defined in manifest
	if sfs == nil {
		sfs, err := i.toStatefulSet(refResource)
		if err != nil {
			return errors.Wrap(err, "hpa interceptor failed to convert unstructured to statefulset")
		}
		if i.isReplicaInRange(sfs.Spec.Replicas, minReplicas, maxReplicas) {
			i.logger.Warnf("HPA reconciler detected manifest inconsistency: replicas of deployment '%s@%s' "+
				"is %d but range of replicas in HPA is %d-%d (min-max)",
				sfs.GetName(), refNamespace, *sfs.Spec.Replicas, minReplicas, maxReplicas)
		}
		return nil
	}

	//statefulset exists in cluster: update replicas field in manifest
	err = unstructured.SetNestedField(refResource.Object, sfs.Spec.Replicas, "spec", "replicas")
	if err != nil {
		i.logger.Errorf("Failed to set replica count of the statefulset '%s' (namespace: %s) "+
			"referenced by an HPA: %s", refResource.GetName(), refNamespace, err)
	}
	return err
}

func (i *HPAInterceptor) isReplicaInRange(given *int32, minReplicas int32, maxReplicas int32) bool {
	return given == nil && minReplicas != 1 || *given < minReplicas || *given > maxReplicas
}

func (i *HPAInterceptor) toDeployment(unstruct *unstructured.Unstructured) (*v1.Deployment, error) {
	deploy := &v1.Deployment{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstruct.Object, deploy)
	return deploy, err
}

func (i *HPAInterceptor) toStatefulSet(unstruct *unstructured.Unstructured) (*v1.StatefulSet, error) {
	sfs := &v1.StatefulSet{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstruct.Object, sfs)
	return sfs, err
}
