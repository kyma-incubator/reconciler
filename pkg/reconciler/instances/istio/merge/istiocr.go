package merge

import (
	"context"
	"encoding/json"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/clientset"
	"github.com/kyma-project/istio/operator/api/v1alpha1"
	"github.com/kyma-project/istio/operator/pkg/lib/gatherer"
	istioOperator "istio.io/istio/operator/pkg/apis/istio/v1alpha1"
	apiMeta "k8s.io/apimachinery/pkg/api/meta"
)

func getIstioCR(ctx context.Context, provider clientset.Provider, kubeConfig string) (*v1alpha1.IstioList, error) {
	client, err := provider.GetIstioClient(kubeConfig)
	if err != nil {
		return nil, err
	}
	istioCRList, err := gatherer.ListIstioCR(ctx, client)
	if err != nil && !apiMeta.IsNoMatchError(err) {
		return nil, err
	}

	return istioCRList, nil
}

func applyIstioCR(istioCRList *v1alpha1.IstioList, operatorManifest string) (string, error) {
	toBeInstalledIop := istioOperator.IstioOperator{}
	err := json.Unmarshal([]byte(operatorManifest), &toBeInstalledIop)
	if err != nil {
		return "", err
	}

	for _, cr := range istioCRList.Items {
		_, err := cr.MergeInto(toBeInstalledIop)
		if err != nil {
			return "", err
		}
	}

	outputManifest, err := json.Marshal(toBeInstalledIop)
	if err != nil {
		return "", err
	}

	return string(outputManifest), nil
}
