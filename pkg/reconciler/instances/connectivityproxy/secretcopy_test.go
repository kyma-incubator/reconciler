package connectivityproxy

import (
	"context"
	"github.com/stretchr/testify/require"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
)

func TestSecretCopy(t *testing.T) {
	t.Run("Should create secret with target clientset", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()
		secretCopy := SecretCopy{
			Namespace:       "namespace",
			Name:            "name",
			targetClientSet: clientset,
			from:            &mockSecretFrom{},
		}

		err := secretCopy.Transfer()
		require.NoError(t, err)

		created, err := clientset.
			CoreV1().
			Secrets("namespace").
			Get(context.Background(), "name", metav1.GetOptions{})

		require.NoError(t, err)
		require.NotNil(t, created)
		require.Equal(t, "name", created.Name)
		require.Equal(t, "namespace", created.Namespace)
	})
}

type mockSecretFrom struct {
}

func (msf *mockSecretFrom) Get() (*coreV1.Secret, error) {
	toCopy := &coreV1.Secret{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "test-namespace",
		},
		Immutable: nil,
		Data: map[string][]byte{
			"token": []byte("tokenValue"),
		},
		StringData: nil,
		Type:       "",
	}

	return toCopy, nil
}
