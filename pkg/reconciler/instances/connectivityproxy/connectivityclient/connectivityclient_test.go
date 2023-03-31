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

func TestNewConnectivityClient(t *testing.T) {

	t.Run("Should return error when nil configuration is provided", func(t *testing.T) {

		_, err := NewConnectivityCAClient(nil)

		require.Error(t, err)
	})

	t.Run("Should return error when bad configuration is provided - missing url", func(t *testing.T) {

		badConfig := map[string]interface{}{
			"global.binding.CAs_path": "/api/v1/CAs/signing",
		}

		_, err := NewConnectivityCAClient(badConfig)

		require.Error(t, err)
	})

	t.Run("Should return error when bad configuration is provided - missing CA path", func(t *testing.T) {

		badConfig := map[string]interface{}{
			"global.binding.url": "cf.test-address.sap.com",
		}

		_, err := NewConnectivityCAClient(badConfig)
		require.Error(t, err)
	})

	t.Run("Should return error when bad configuration is provided - empty url", func(t *testing.T) {

		badConfig := map[string]interface{}{
			"global.binding.url":      "",
			"global.binding.CAs_path": "/api/v1/CAs/signing",
		}

		_, err := NewConnectivityCAClient(badConfig)

		require.Error(t, err)
	})

	t.Run("Should return error when bad configuration is provided - empty CA path", func(t *testing.T) {

		badConfig := map[string]interface{}{
			"global.binding.url":      "cf.test-address.sap.com",
			"global.binding.CAs_path": "",
		}

		_, err := NewConnectivityCAClient(badConfig)
		require.Error(t, err)
	})

	t.Run("Should correct ConnectivityCAClient when provided with correct configuration", func(t *testing.T) {

		goodConfig := map[string]interface{}{
			"global.binding.url":      "cf.test-address.sap.com",
			"global.binding.CAs_path": "/api/v1/CAs/signing",
		}

		clientCA, err := NewConnectivityCAClient(goodConfig)

		require.NoError(t, err)
		require.Equal(t, "cf.test-address.sap.com/api/v1/CAs/signing", clientCA.url)
		require.NotNil(t, clientCA.client)
	})
}
