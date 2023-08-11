package service

import (
	"context"
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type PVCInterceptor struct {
	kubeClient kubernetes.Client
	logger     *zap.SugaredLogger
}

func (i *PVCInterceptor) Intercept(resources *kubernetes.ResourceCacheList, namespace string) error {
	return i.interceptPVC(resources, namespace)
}

func (i *PVCInterceptor) interceptPVC(resources *kubernetes.ResourceCacheList, namespace string) error {
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

		originalStorage := pvcOriginal.Spec.Resources.Requests.Storage()
		targetStorage := pvcTarget.Spec.Resources.Requests.Storage()

		if !originalStorage.Equal(*targetStorage) {
			i.logger.Warnf("Size of PVC '%s' (namespace: %s) has changed from %s to %s: Adjusting requested size to %s",
				u.GetName(), namespace, targetStorage, originalStorage, originalStorage)

			err = unstructured.SetNestedField(u.Object, originalStorage.String(), "spec", "resources", "requests", "storage")
			if err != nil {
				i.logger.Errorf("Failed to set requested storage of the pvc '%s' (namespace: %s) : %s", u.GetName(), namespace, err)
			}
		}
		return err
	}

	return resources.VisitByKind("PersistentVolumeClaim", interceptorFunc)
}
