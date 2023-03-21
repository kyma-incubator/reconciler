package connectivityproxy

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
)

func TestFromSecret(t *testing.T) {
	t.Run("Should read secret from default namespace", func(t *testing.T) {
		expected := v1.Secret{
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

		fromSecret := FromSecret{
			Namespace: "test-namespace",
			Name:      "test-name",
			inCluster: fake.NewSimpleClientset(&expected),
		}

		actual, err := fromSecret.Get()
		assert.NoError(t, err)
		assert.Equal(t, expected, *actual)
	})
}
