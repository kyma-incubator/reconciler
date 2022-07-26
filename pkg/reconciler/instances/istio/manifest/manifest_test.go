package manifest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	istioOperator "istio.io/istio/operator/pkg/apis/istio/v1alpha1"
)

const (
	istioManifest = `
apiVersion: version/v1
kind: Kind1
metadata:
  namespace: namespace
  name: name
---
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  namespace: namespace
  name: name
---
apiVersion: version/v2
kind: Kind2
metadata:
  namespace: namespace
  name: name
`
)

func Test_extractIstioOperatorContextFrom(t *testing.T) {

	t.Run("should not extract istio operator from manifest that does not contain istio operator", func(t *testing.T) {
		// when
		result, err := ExtractIstioOperatorContextFrom("")

		operator := istioOperator.IstioOperator{}

		json.Unmarshal([]byte(result), &operator)

		// then
		require.Empty(t, result)
		require.Error(t, err)
		require.Contains(t, err.Error(), "could not be found")
	})

	t.Run("should extract istio operator from combo manifest", func(t *testing.T) {
		// when
		result, err := ExtractIstioOperatorContextFrom(istioManifest)

		// then
		require.NoError(t, err)
		require.Contains(t, result, "IstioOperator")
	})

}
