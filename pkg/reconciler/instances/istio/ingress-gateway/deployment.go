package ingressgateway

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	istioCR "github.com/kyma-project/istio/operator/api/v1alpha1"
)

const (
	namespace      string = "istio-system"
	name           string = "istio-ingressgateway"
	annotationName string = "reconciler.kyma-project.io/lastRestartDate"
)

func RestartDeployment(ctx context.Context, k8sClient client.Client) error {
	deployment := appsv1.Deployment{}
	err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &deployment)
	if err != nil {
		return err
	}

	if len(deployment.Spec.Template.Annotations) == 0 {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}
	deployment.Spec.Template.Annotations[annotationName] = time.Now().Format(time.RFC3339)

	return k8sClient.Update(ctx, &deployment)
}

func NeedsRestart(ctx context.Context, client client.Client, istioCRList *istioCR.IstioList) (bool, error) {
	numTrustedProxies, err := GetNumTrustedProxyFromIstioCM(ctx, client)
	if err != nil {
		return false, err
	}

	// Restart to default if no IstioCR is present
	if numTrustedProxies != nil && (istioCRList == nil || len(istioCRList.Items) == 0) {
		return true, nil
	}

	if istioCRList == nil {
		return false, nil
	}

	for _, cr := range istioCRList.Items {
		isNewNotNil := (cr.Spec.Config.NumTrustedProxies != nil)
		isOldNotNil := (numTrustedProxies != nil)
		if isNewNotNil && isOldNotNil {
			if *cr.Spec.Config.NumTrustedProxies != *numTrustedProxies {
				return true, nil
			}
		}
		if isNewNotNil != isOldNotNil {
			return true, nil
		}
	}
	return false, nil
}
