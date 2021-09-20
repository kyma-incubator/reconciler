package connectivityproxy

import (
	"fmt"
	coreV1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFromURL(t *testing.T) {
	t.Run("Should create secret with content from url", func(t *testing.T) {
		key := "key"
		value := "Secret value"

		server := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, err := fmt.Fprintln(w, value)
				if err != nil {
					return
				}
			}))
		defer server.Close()

		from := FromURL{
			URL: server.URL,
			Key: key,
		}

		secret, err := from.Get()
		require.NoError(t, err)
		expected := coreV1.Secret{
			TypeMeta:   v1.TypeMeta{Kind: "Secret"},
			ObjectMeta: v1.ObjectMeta{},
			Data: map[string][]byte{
				key: []byte(value + "\n"),
			},
			StringData: nil,
			Type:       coreV1.SecretTypeOpaque,
		}
		require.Equal(t, expected, *secret)
	})

}
