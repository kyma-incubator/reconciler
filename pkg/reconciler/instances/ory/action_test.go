package ory

import (
	"context"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/ory/jwks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestOryJwksSecret(t *testing.T) {
	tests := []struct {
		Name            string
		PreCreateSecret bool
	}{
		{
			Name:            "Secret to patch does not exist",
			PreCreateSecret: false,
		},
		{
			Name:            "Secret was patched successfully",
			PreCreateSecret: true,
		},
	}
	for _, testCase := range tests {
		test := testCase
		t.Run(test.Name, func(t *testing.T) {
			logger := zaptest.NewLogger(t).Sugar()
			a := ReconcileAction{
				step: "test-jwks-secret",
			}
			name := types.NamespacedName{Name: "test-jwks-secret", Namespace: "test"}
			ctx := context.Background()
			k8sClient := fake.NewSimpleClientset()
			var existingUID types.UID

			patchData, err := jwks.PreparePatchData(jwksAlg, jwksBits)
			require.NoError(t, err)

			if test.PreCreateSecret {
				existingSecret, err := preCreateSecret(ctx, k8sClient, name)
				assert.NoError(t, err)
				existingUID = existingSecret.UID
			}

			err = a.patchSecret(ctx, k8sClient, name, patchData, logger)
			if !test.PreCreateSecret {
				assert.NotNil(t, err)
			} else {
				assert.NoError(t, err)

				secret, err := k8sClient.CoreV1().Secrets(name.Namespace).Get(ctx, name.Name, metav1.GetOptions{})
				require.NoError(t, err)
				assert.Equal(t, name.Name, secret.Name)
				assert.Equal(t, name.Namespace, secret.Namespace)
				assert.NotNil(t, secret.Data)

				assert.Equal(t, existingUID, secret.UID)
			}

		})
	}
}

func preCreateSecret(ctx context.Context, client kubernetes.Interface, name types.NamespacedName) (*v1.Secret, error) {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
		Data: map[string][]byte{},
	}

	return client.CoreV1().Secrets(name.Namespace).Create(ctx, secret, metav1.CreateOptions{})
}
