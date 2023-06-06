package connectivityproxy

import (
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
)

func TestRunner(t *testing.T) {
	t.Run("Should register Connectivity Proxy reconciler", func(t *testing.T) {
		reconciler, err := service.GetReconciler("connectivity-proxy")
		require.NoError(t, err)
		require.NotNil(t, reconciler)
	})
}

func Test_newEncodedSecretSvcKey(t *testing.T) {
	secretRootKey := "testmeplz"
	data := map[string][]byte{
		secretRootKey: []byte("test-secret-root-content"),
	}

	actual, err := newEncodedSecretSvcKey(secretRootKey, &v1.Secret{
		Data: data,
	})
	require.NoError(t, err)
	require.Equal(t, string(data[secretRootKey]), actual)
}
