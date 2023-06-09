package secrets

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSecretRepository(t *testing.T) {

	t.Run("Should make and save connectivity proxy secret", func(t *testing.T) {

		fakeClientSet := fake.NewSimpleClientset()
		repo := NewSecretRepo("test-namespace", fakeClientSet)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := repo.SaveSecretCpSvcKey(ctx, "test", "me")
		require.NoError(t, err)

		secret, err := fakeClientSet.CoreV1().Secrets("test-namespace").Get(ctx, "test", metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, "me", string(secret.Data["service_key"]))
	})

	t.Run("Should make and save mapping operator secret", func(t *testing.T) {

		fakeClientSet := fake.NewSimpleClientset()
		repo := NewSecretRepo("test-namespace", fakeClientSet)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		data, err := repo.SaveSecretMappingOperator(ctx, "test", []byte("me"), []byte("plz"))
		require.NoError(t, err)

		secret, err := fakeClientSet.CoreV1().Secrets("test-namespace").Get(ctx, "test", metav1.GetOptions{})
		require.NoError(t, err)

		require.Equal(t, data, secret.Data)
	})

	t.Run("Should make and save CA secret from ca bytes in desired namespace", func(t *testing.T) {

		fakeClientSet := fake.NewSimpleClientset()
		repo := NewSecretRepo("test-namespace", fakeClientSet)

		caBytes := []byte("cacert-value")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := repo.SaveIstioCASecret(ctx, "secret-name", "ca-cert", caBytes)
		require.NoError(t, err)

		secret, err := fakeClientSet.CoreV1().Secrets("test-namespace").Get(ctx, "secret-name", metav1.GetOptions{})

		require.NoError(t, err)

		value, ok := secret.Data["ca-cert"]

		require.NoError(t, err)
		require.Equal(t, true, ok)
		require.Equal(t, []byte("cacert-value"), value)
	})

	t.Run("Should replace previous secret and with new one storing ca bytes", func(t *testing.T) {
		// given
		fakeClientSet := fake.NewSimpleClientset()

		oldCACert := []byte("old-cacert-value")
		newCACert := []byte("new-cacert-value")
		namespace := "test-namespace"
		secretName := "secret-name"
		secretKey := "ca-cert"

		existingSecret := &coreV1.Secret{
			TypeMeta: metav1.TypeMeta{Kind: "Secret"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
			},
			Data: map[string][]byte{
				secretKey: oldCACert,
			},
			StringData: nil,
			Type:       coreV1.SecretTypeOpaque,
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		_, err := fakeClientSet.CoreV1().Secrets(namespace).Create(ctx, existingSecret, metav1.CreateOptions{})
		require.NoError(t, err)

		repo := NewSecretRepo(namespace, fakeClientSet)

		// when
		err = repo.SaveIstioCASecret(ctx, secretName, secretKey, newCACert)
		require.NoError(t, err)

		// then
		secret, err := fakeClientSet.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})

		require.NoError(t, err)

		value, ok := secret.Data["ca-cert"]
		require.NoError(t, err)
		require.Equal(t, true, ok)
		require.Equal(t, newCACert, value)
	})
}
