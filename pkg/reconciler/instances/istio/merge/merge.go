package merge

import (
	"context"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/clientset"
	"go.uber.org/zap"
)

// IstioOperatorConfiguration merges default Kyma Istio Operator file with user configuration in Istio CR and in Istio ConfigMap.
// If there is no IstioCRD or there are no Istio CR/ConfigMap present on the cluster, it defaults to the operator file.
func IstioOperatorConfiguration(ctx context.Context, provider clientset.Provider, operatorManifest string, kubeConfig string, logger *zap.SugaredLogger) (string, error) {
	istioCRList, err := getIstioCR(ctx, provider, kubeConfig)
	if err != nil {
		return "", err
	}

	kubeClient, err := provider.RetrieveFrom(kubeConfig, logger)
	if err != nil {
		return "", err
	}

	cniEnabled, err := GetCNIConfigMap(ctx, kubeClient)
	if err != nil {
		return "", err
	}

	if cniEnabled != "" {
		operatorManifest, err = applyIstioCNI(cniEnabled, operatorManifest)
		if err != nil {
			logger.Error("Could not apply Istio CNI ConfigMap into Istio Operator")
			return "", err
		}
		logger.Debugf("Istio CNI ConfigMap was applied to the Istio Operator configuration")
	}

	if istioCRList != nil {
		operatorManifest, err = applyIstioCR(istioCRList, operatorManifest)
		if err != nil {
			return "", err
		}
		logger.Debugf("Istio CRs were applied to the Istio Operator configuration")
	}

	logger.Debugf("No Istio CRs found on the cluster, applying default configuration")
	return operatorManifest, nil
}
