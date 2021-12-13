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

func (i *PVCInterceptor) Intercept(resources *kubernetes.ResourceList, namespace string) error {
	interceptorFunc := func(u *unstructured.Unstructured) error {
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
			return errors.Wrap(err, fmt.Sprintf("pvc interceptor failed to convert unstructured entity '%s'",
				u.GetName()))
		}

		targetStorage := pvcTarget.Spec.Resources.Requests.Storage()
		if pvcOriginal.Spec.Resources.Requests.Storage().Equal(*targetStorage) {
			resources.Remove(u) //no change in storage size, remove pvc from reconciliation-scope
			i.logger.Debugf("Removing PVC '%s' from reconciliation scope because storage-size (%s) hasn't changed",
				u.GetName(), targetStorage)
		}

		return nil

	}

	return resources.VisitByKind("PersistentVolumeClaim", interceptorFunc)
}
