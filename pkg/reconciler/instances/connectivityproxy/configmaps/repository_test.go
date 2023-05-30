package configmaps

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestConfigmapRepository(t *testing.T) {

	t.Run("Should create service mapping configmap", func(t *testing.T) {

		fakeClientSet := fake.NewSimpleClientset()
		repo := NewConfigMapRepo("test-namespace", fakeClientSet)

		ctx := context.Background()

		err := repo.CreateServiceMappingConfig(ctx, "test")
		require.NoError(t, err)

		_, err = fakeClientSet.CoreV1().ConfigMaps("test-namespace").Get(context.Background(), "test", metav1.GetOptions{})
		require.NoError(t, err)
	})

	t.Run("Should not replace already existing configmap", func(t *testing.T) {
		// given
		fakeClientSet := fake.NewSimpleClientset()

		namespace := "test-namespace"
		secretName := "secret-name"
		expectedData := map[string]string{
			"test": "me",
		}

		existingConfigmap := &coreV1.ConfigMap{
			TypeMeta: metav1.TypeMeta{Kind: "ConfigMap"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
			},
			Data: expectedData,
		}

		_, err := fakeClientSet.CoreV1().
			ConfigMaps(namespace).
			Create(context.Background(), existingConfigmap, metav1.CreateOptions{})

		require.NoError(t, err)

		repo := NewConfigMapRepo(namespace, fakeClientSet)

		ctx := context.Background()

		// when
		err = repo.CreateServiceMappingConfig(ctx, secretName)
		require.NoError(t, err)

		// then
		actual, err := fakeClientSet.CoreV1().ConfigMaps(namespace).Get(context.Background(), secretName, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, expectedData, actual.Data)
	})
}
