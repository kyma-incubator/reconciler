package cni

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/clientset"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"helm.sh/helm/v3/pkg/chart/loader"
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

type cniValues struct {
	Components struct {
		CNI struct {
			Enabled bool `json:"enabled"`
		} `json:"cni"`
	} `json:"components"`
}

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

// GetCNIState checks if CNI is enabled on Istio Operator installed on the cluster and returns its value.
func GetCNIState(dynamicClient dynamic.Interface) (bool, error) {
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

	json.Unmarshal(jsonSlice, &operator)
	return &operator, nil
}

func isCniEnabledInOperatorChart(workspace chart.Factory, branch string, istioChart string) (bool, error) {
	ws, err := workspace.Get(branch)
	if err != nil {
		return false, err
	}

	istioHelmChart, err := loader.Load(filepath.Join(ws.ResourceDir, istioChart))
	if err != nil {
		return false, err
	}

	mapAsJSON, err := json.Marshal(istioHelmChart.Values)
	if err != nil {
		return false, err
	}
	var cniValues cniValues

	err = json.Unmarshal(mapAsJSON, &cniValues)
	if err != nil {
		return false, err
	}

	return cniValues.Components.CNI.Enabled, nil
}

func IsCNIRolloutRequired(context context.Context, kubeClient kubernetes.Interface, dynamicClient dynamic.Interface, workspace chart.Factory, branchVersion string, istioChart string, logger *zap.SugaredLogger) (bool, error) {
	desiredCniState, err := isCniEnabledInOperatorChart(workspace, branchVersion, istioChart)
	if err != nil {
		logger.Error("Could not retrieve default Istio CNI plugin setting")
		return false, err
	}

	actualCniState, err := GetCNIState(dynamicClient)
	if err != nil {
		logger.Error("Could not retrieve default Istio CNI plugin setting")
		return false, err
	}

	return desiredCniState != actualCniState, nil

}
