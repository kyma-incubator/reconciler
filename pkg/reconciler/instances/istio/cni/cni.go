package cni

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/clientset"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/wrapperspb"
	istioOperator "istio.io/istio/operator/pkg/apis/istio/v1alpha1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

const (
	kymaNamespace     = "kyma-system"
	configMapCNI      = "kyma-istio-cni"
	istioOperatorName = "installed-state-default-operator"
	istioNamespace    = "istio-system"
)

// ApplyCNIConfiguration applies CNI configuration from kyma-istio-cni ConfigMap to the Istio Operator.
// If there is no such ConfigMap, it defaults to the operator file.
func ApplyCNIConfiguration(ctx context.Context, provider clientset.Provider, operatorManifest string, kubeConfig string, logger *zap.SugaredLogger) (string, error) {
	kubeClient, err := provider.RetrieveFrom(kubeConfig, logger)
	if err != nil {
		return "", err
	}

	cniEnabled, err := getCNIConfigMapValue(ctx, kubeClient)
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

	logger.Debugf("No Istio CNI found on the cluster, applying default configuration")

	return operatorManifest, nil
}

// getCNIState checks if CNI is enabled on Istio Operator installed on the cluster and returns its value.
func getCNIState(provider clientset.Provider, kubeConfig string, logger *zap.SugaredLogger) (bool, error) {
	dynamicClient, err := provider.GetDynamicClient(kubeConfig)
	if err != nil {
		logger.Error("Could not retrieve Dynamic client from Kubeconfig!")
		return false, err
	}

	iop, err := getIstioOperator(dynamicClient)
	if err != nil {
		return false, nil
	}
	actualCniState := iop.Spec.Components.Cni.Enabled.GetValue()
	return actualCniState, nil
}

func getCNIConfigMapValue(ctx context.Context, clientSet kubernetes.Interface) (string, error) {
	cm, err := clientSet.CoreV1().ConfigMaps(kymaNamespace).Get(ctx, configMapCNI, metav1.GetOptions{})
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

func getIstioOperator(dynamicClient dynamic.Interface) (*istioOperator.IstioOperator, error) {
	obj, err := dynamicClient.Resource(schema.GroupVersionResource{Group: "install.istio.io", Version: "v1alpha1", Resource: "istiooperators"}).Namespace(istioNamespace).Get(context.Background(), istioOperatorName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("default Kyma IstioOperator CR wasn't found err=%s", err)
	}

	jsonSlice, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	operator := istioOperator.IstioOperator{}

	err = json.Unmarshal(jsonSlice, &operator)
	if err != nil {
		return nil, err
	}

	return &operator, nil
}

func isCniEnabledInOperatorChart(istioChart string) (bool, error) {
	toBeInstalledIop := istioOperator.IstioOperator{}
	err := json.Unmarshal([]byte(istioChart), &toBeInstalledIop)
	if err != nil {
		return false, err
	}

	return toBeInstalledIop.Spec.Components.Cni.Enabled.GetValue(), nil
}

func IsCNIRolloutRequired(provider clientset.Provider, kubeConfig string, istioChart string, logger *zap.SugaredLogger) (bool, error) {

	desiredCniState, err := isCniEnabledInOperatorChart(istioChart)
	if err != nil {
		logger.Error("Could not retrieve chart Istio CNI plugin setting. Falling back to no CNI rollout required.")
		return false, err
	}

	actualCniState, err := getCNIState(provider, kubeConfig, logger)
	if err != nil {
		logger.Error("Could not retrieve actual CNI state. Falling back to no CNI rollout required.")
		return false, err
	}

	return desiredCniState != actualCniState, nil

}
