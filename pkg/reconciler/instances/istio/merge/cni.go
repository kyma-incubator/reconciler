package merge

import (
	"context"
	"encoding/json"
	"strconv"

	"google.golang.org/protobuf/types/known/wrapperspb"
	istioOperator "istio.io/istio/operator/pkg/apis/istio/v1alpha1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	KymaNamespace = "kyma-system"
	ConfigMapCNI  = "kyma-istio-cni"
)

// GetCNIConfigMap fetches ConfigMap with Istio CNI overrides and returns a value from required key.
// Returns empty string if ConfigMap is invalid or is not present on the cluster.
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
