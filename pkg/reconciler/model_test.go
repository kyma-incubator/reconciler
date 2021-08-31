package reconciler

import (
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes/fake"
	"testing"
)

func TestStatusUpdater(t *testing.T) {
	t.Parallel()

	t.Run("Should parse configs", func(t *testing.T) {
		model := Reconciliation{
			Configuration: []Configuration{
				{
					Key:   "test1",
					Value: "value1",
				},
				{
					Key:   "test2",
					Value: "value2",
				},
			},
		}

		configsMap := model.ConfigsToMap()
		assert.Equal(t, map[string]string{
			"test1": "value1",
			"test2": "value2",
		}, configsMap)
	})

	t.Run("Should read correct token secret", func(t *testing.T) {
		client := initiateClient()

		repo := Repo{
			URL:   "https://localhost",
			Token: "",
		}

		err := repo.ReadToken(client.CoreV1(), "default")

		assert.NoError(t, err)
		assert.Equal(t, "tokenValue", repo.Token)
	})

	t.Run("Should read return error when token secret not found", func(t *testing.T) {
		client := initiateClient()

		repo := Repo{
			URL:   "https://localhost",
			Token: "",
		}

		err := repo.ReadToken(client.CoreV1(), "non-existing")

		assert.Error(t, err)
	})

	t.Run("Should read correct token secret", func(t *testing.T) {
		assertParsed(t, "localhost", "localhost", true)
		assertParsed(t, "localhost", "localhost:8080", true)
		assertParsed(t, "localhost", "http://localhost:8080", false)
		assertParsed(t, "localhost", "www.localhost:8080", true)
		assertParsed(t, "localhost", "https://www.localhost:8080", false)
		assertParsed(t, "192.168.1.2", "192.168.1.2", true)
		assertParsed(t, "192.168.1.2", "192.168.1.2:8080", true)
	})
}

func initiateClient() *fake.Clientset {
	client := fake.NewSimpleClientset(&v1.Secret{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "localhost",
			Namespace: "default",
		},
		Immutable: nil,
		Data: map[string][]byte{
			"token": []byte("tokenValue"),
		},
		StringData: nil,
		Type:       "",
	})
	return client
}

func assertParsed(t *testing.T, expected string, url string, expectError bool) {
	key, err := MapSecretKey(url)
	if expectError {
		assert.Error(t, err)
	} else {
		assert.NoError(t, err)
		assert.Equal(t, expected, key)
	}
}
