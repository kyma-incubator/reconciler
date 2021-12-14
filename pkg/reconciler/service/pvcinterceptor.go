package service

import (
	"context"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	appv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type PVCInterceptor struct {
	kubeClient kubernetes.Client
	logger     *zap.SugaredLogger
}

func (i *PVCInterceptor) Intercept(resources *kubernetes.ResourceList, namespace string) error {
	err := i.interceptPVC(resources, namespace)
	if err != nil {
		return err
	}
	err = i.interceptStatefulSet(resources, namespace)
	return err
}

func (i *PVCInterceptor) interceptPVC(resources *kubernetes.ResourceList, namespace string) error {
	interceptorFunc := func(u *unstructured.Unstructured) error {
		namespace := kubernetes.ResolveNamespace(u, namespace)

		pvcOriginal, err := i.kubeClient.GetPersistentVolumeClaim(context.Background(), u.GetName(), namespace)
		if err != nil {
			return err
		}

		if pvcOriginal == nil { //PVC does not exist yet: nothing to do
			return nil
		}

		//convert unstruct to PVC resource
		pvcTarget := &v1.PersistentVolumeClaim{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, pvcTarget); err != nil {
			return errors.Wrap(err, fmt.Sprintf("pvc interceptor failed to convert unstructured entity '%s' to PVC",
				u.GetName()))
		}

		i.logger.Debugf("Removing PVC '%s' (namespace: %s) from reconciliation scope: "+
			"PVC already exists but PVC reconciliation is not supported yet", u.GetName(), namespace)
		resources.Remove(u)

		//notify about PVC inconsistencies
		i.checkForInconsistentPVC(u, namespace, pvcOriginal, pvcTarget)

		return nil
	}

	return resources.VisitByKind("PersistentVolumeClaim", interceptorFunc)
}

func (i *PVCInterceptor) interceptStatefulSet(resources *kubernetes.ResourceList, namespace string) error {
	interceptorFunc := func(u *unstructured.Unstructured) error {
		namespace := kubernetes.ResolveNamespace(u, namespace)

		sfsOriginal, err := i.kubeClient.GetStatefulSet(context.Background(), u.GetName(), namespace)
		if err != nil {
			return err
		}

		if sfsOriginal == nil { //StatefulSet does not exist yet: nothing to do
			return nil
		}

		//convert unstruct to STS resource
		sfsTarget := &appv1.StatefulSet{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, sfsTarget); err != nil {
			return errors.Wrap(err, fmt.Sprintf("pvc interceptor failed to convert unstructured entity '%s' "+
				"to statefulset", u.GetName()))
		}

		if len(sfsOriginal.Spec.VolumeClaimTemplates) > 0 {
			i.logger.Debugf("Removing statefulset '%s' (namespace: %s) from reconciliation scope: "+
				"statefulset already exists and has PVC defined but PVC reconciliation is not supported yet",
				u.GetName(), namespace)
			resources.Remove(u)
		}

		//notify about PVC inconsistencies
		pvcExisting := len(sfsOriginal.Spec.VolumeClaimTemplates)
		pvcExpected := len(sfsTarget.Spec.VolumeClaimTemplates)
		if pvcExisting != pvcExpected {
			i.logger.Warnf("Number of defined PVCs in statefulset '%s' (namespace %s) are different: "+
				"%d exist and %d expected but PVC reconciliation is not supported yet",
				sfsOriginal.GetName(), namespace, pvcExisting, pvcExpected)
		}

		for _, pvcOriginal := range sfsOriginal.Spec.VolumeClaimTemplates {
			pvcTarget := i.getPVC(pvcOriginal.GetName(), pvcOriginal.GetNamespace(), sfsTarget.Spec.VolumeClaimTemplates)
			if pvcTarget == nil {
				i.logger.Warnf("PVC '%s' (namespace: %s) no longer exists in manifest: "+
					"PVC deletion is not supported yet", pvcOriginal.GetName(), pvcOriginal.GetNamespace())
			} else {
				i.checkForInconsistentPVC(u, namespace, &pvcOriginal, pvcTarget)
			}
		}

		return nil
	}

	return resources.VisitByKind("StatefulSet", interceptorFunc)
}

func (i *PVCInterceptor) checkForInconsistentPVC(u *unstructured.Unstructured, namespace string, pvcOriginal *v1.PersistentVolumeClaim, pvcTarget *v1.PersistentVolumeClaim) {
	originalStorage := pvcOriginal.Spec.Resources.Requests.Storage()
	targetStorage := pvcTarget.Spec.Resources.Requests.Storage()
	if !originalStorage.Equal(*targetStorage) {
		i.logger.Warnf("Size of PVC '%s' (namespace: %s) has changed from %s to %s: "+
			"reconciliation of PVC is not supported yet",
			u.GetName(), namespace, originalStorage, targetStorage)
	}
}

func (i *PVCInterceptor) getPVC(name, namespace string, pvcs []v1.PersistentVolumeClaim) *v1.PersistentVolumeClaim {
	for _, pvc := range pvcs {
		if pvc.GetName() == name && (pvc.GetNamespace() == "" || pvc.GetNamespace() == namespace) {
			return &pvc
		}
	}
	return nil
}
