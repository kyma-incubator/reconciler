package manifest

import (
	"errors"
	"strings"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
)

const (
	istioOperatorKind = "IstioOperator"
)

//Returns a manifest with IstioOperator CR excluded. The given manifest must be in YAML format.
func GenerateNewManifestWithoutIstioOperatorFrom(manifest string) (string, error) {
	unstructs, err := kubernetes.ToUnstructured([]byte(manifest), true)
	if err != nil {
		return "", err
	}

	builder := strings.Builder{}
	for _, unstruct := range unstructs {
		if unstruct.GetKind() == istioOperatorKind {
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

//Returns IstioOperator CR, if present in the given manifest. Returns an error otherwise. The given manifest must be in YAML format.
func ExtractIstioOperatorContextFrom(manifest string) (string, error) {
	unstructs, err := kubernetes.ToUnstructured([]byte(manifest), true)
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
