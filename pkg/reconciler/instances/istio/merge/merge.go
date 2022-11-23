package merge

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/clientset"
	"github.com/kyma-project/istio/operator/api/v1alpha1"
	"github.com/kyma-project/istio/operator/pkg/lib/gatherer"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/wrapperspb"
	istioOperator "istio.io/istio/operator/pkg/apis/istio/v1alpha1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	apiMeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	KymaNamespace = "kyma-system"
	ConfigMapCNI  = "kyma-istio-cni"
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
		combinedManifest, err := applyIstioCNI(cniEnabled, operatorManifest)
		if err != nil {
			logger.Error("Could not apply Istio CNI ConfigMap into Istio Operator")
			return "", err
		}

		logger.Debugf("Istio CNI ConfigMap was applied to the Istio Operator configuration")
		return combinedManifest, nil
	}

	if istioCRList != nil {
		combinedManifest, err := applyIstioCR(istioCRList, operatorManifest)
		if err != nil {
			return "", err
		}

		logger.Debugf("Istio CRs were applied to the Istio Operator configuration")
		return combinedManifest, nil
	}

	logger.Debugf("No Istio CRs found on the cluster, applying default configuration")
	return operatorManifest, nil
}

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

// GetCNIConfigMap fetches ConfigMap with Istio CNI overrides and returns a value from required key
func GetCNIConfigMap(ctx context.Context, clientSet kubernetes.Interface) (string, error) {
	cm, err := clientSet.CoreV1().ConfigMaps(KymaNamespace).Get(ctx, ConfigMapCNI, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return "", nil
		}
		return "", err
	}

	cniEnabled, ok := cm.Data["enabled"]
	if !ok {
		return "", nil
	}

	return cniEnabled, nil
}

func applyIstioCNI(cmValue string, operatorManifest string) (string, error) {
	toBeInstalledIop := istioOperator.IstioOperator{}
	err := json.Unmarshal([]byte(operatorManifest), &toBeInstalledIop)
	if err != nil {
		return "", err
	}

	cniEnabled, err := strconv.ParseBool(cmValue)
	if err != nil {
		return "", err
	}
	toBeInstalledIop.Spec.Components.Cni.Enabled = wrapperspb.Bool(cniEnabled)

	outputManifest, err := json.Marshal(toBeInstalledIop)
	if err != nil {
		return "", err
	}
	return string(outputManifest), nil
}
