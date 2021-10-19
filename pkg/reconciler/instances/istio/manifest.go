package istio

import (
	"errors"
	"strings"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/kubeclient"
)

const (
	istioOperatorKind = "IstioOperator"
)

func generateNewManifestWithoutIstioOperatorFrom(manifest string) (string, error) {
	unstructs, err := kubeclient.ToUnstructured([]byte(manifest), true)
	if err != nil {
		return "", err
	}

	builder := strings.Builder{}
	for _, unstruct := range unstructs {
		if unstruct.GetKind() == "IstioOperator" {
			continue
		}

		unstructBytes, err := unstruct.MarshalJSON()
		if err != nil {
			return "", err
		}

		builder.WriteString("---\n")
		builder.WriteString(string(unstructBytes))
	}

	return builder.String(), nil
}

func extractIstioOperatorContextFrom(manifest string) (string, error) {
	unstructs, err := kubeclient.ToUnstructured([]byte(manifest), true)
	if err != nil {
		return "", err
	}

	for _, unstruct := range unstructs {
		if unstruct.GetKind() != istioOperatorKind {
			continue
		}

		unstructBytes, err := unstruct.MarshalJSON()
		if err != nil {
			return "", nil
		}

		return string(unstructBytes), nil
	}

	return "", errors.New("Istio Operator definition could not be found in manifest")
}
