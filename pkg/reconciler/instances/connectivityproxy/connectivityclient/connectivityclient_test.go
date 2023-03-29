package connectivityclient

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConnectivityClient(t *testing.T) {

	t.Run("Should Get CA Root from url", func(t *testing.T) {
		server := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte("cacert-value"))
				require.NoError(t, err)
			}))
		defer server.Close()

		clientCA := &ConnectivityCAClient{
			url: server.URL,
			client: &http.Client{
				Timeout: 1 * time.Second,
			},
		}

		value, err := clientCA.GetCA()

		require.NoError(t, err)
		require.Equal(t, []byte("cacert-value"), value)
	})

	t.Run("Should return error when server returns code different than 200", func(t *testing.T) {
		server := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
			}))
		defer server.Close()

		clientCA := &ConnectivityCAClient{
			url: server.URL,
			client: &http.Client{
				Timeout: 1 * time.Second,
			},
		}

		_, err := clientCA.GetCA()
		require.Error(t, err)
	})

	t.Run("Should return error when server returns empty CA value", func(t *testing.T) {
		server := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
		defer server.Close()

		clientCA := &ConnectivityCAClient{
			url: server.URL,
			client: &http.Client{
				Timeout: 1 * time.Second,
			},
		}

		_, err := clientCA.GetCA()
		require.Error(t, err)
	})
}

//func TestNewConnectivityClient(t *testing.T) {
//
//	t.Run("Should return error when server returns empty CA value", func(t *testing.T) {
//
//}
