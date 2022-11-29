package merge

import (
	"context"
	"encoding/json"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/clientset"
	ingressgateway "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/ingress-gateway"
	"github.com/kyma-project/istio/operator/api/v1alpha1"
	"github.com/kyma-project/istio/operator/pkg/lib/gatherer"
	"go.uber.org/zap"
	istioOperator "istio.io/istio/operator/pkg/apis/istio/v1alpha1"
	apiMeta "k8s.io/apimachinery/pkg/api/meta"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IstioOperatorConfiguration merges default Kyma Istio Operator file with user configuration in Istio CR.
// If there is no IstioCRD or there are no Istio CR present on the cluster, it defaults to the operator file.
// The returned bool informs of ingress-gateway restart requirement
func IstioOperatorConfiguration(ctx context.Context, provider clientset.Provider, operatorManifest string, kubeConfig string, logger *zap.SugaredLogger) (string, bool, error) {
	istioClient, err := provider.GetIstioClient(kubeConfig)
	if err != nil {
		return "", false, err
	}

	istioCRList, err := getIstioCR(ctx, provider, istioClient)
	if err != nil {
		return "", false, err
	}

	if istioCRList != nil {
		combinedManifest, err := applyIstioCR(istioCRList, operatorManifest)
		if err != nil {
			return "", false, err
		}

		needsRestart, err := ingressgateway.IngressGatewayNeedsRestart(ctx, istioClient, istioCRList)
		if err != nil {
			return "", false, err
		}
		logger.Debugf("Istio CRs were applied to the Istio Operator configuration")
		return combinedManifest, needsRestart, nil
	}

	logger.Debugf("No Istio CRs found on the cluster, applying default configuration")
	return operatorManifest, false, nil
}

func getIstioCR(ctx context.Context, provider clientset.Provider, client client.Client) (*v1alpha1.IstioList, error) {
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
