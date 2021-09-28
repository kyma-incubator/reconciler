package istio

import (
	"github.com/stretchr/testify/require"
	"testing"
)

const (
	istioManifest = `---
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

	istioManifestWithoutIstioOperator = `---
apiVersion: version/v1
kind: Kind1
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

func Test_generateNewManifestWithoutIstioOperatorFrom(t *testing.T) {

	t.Run("should generate empty manifest from empty input manifest", func(t *testing.T) {
		// when
		result := generateNewManifestWithoutIstioOperatorFrom("")

		// then
		require.Empty(t, result)
	})

	t.Run("should return manifest without IstioOperator kind if it was not present ", func(t *testing.T) {
		// when
		result := generateNewManifestWithoutIstioOperatorFrom(istioManifestWithoutIstioOperator)

		// then
		require.Equal(t, result, istioManifestWithoutIstioOperator)
	})

	t.Run("should return manifest without IstioOperator kind if it was present", func(t *testing.T) {
		// when
		result := generateNewManifestWithoutIstioOperatorFrom(istioManifest)

		// then
		require.Equal(t, result, istioManifestWithoutIstioOperator)
	})

}
