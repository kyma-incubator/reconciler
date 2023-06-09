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
	type args struct {
		data          map[string][]byte
		secretRootKey string
	}
	tests := []struct {
		name     string
		args     args
		expected string
	}{
		{
			name: "secret root value",
			args: args{
				data: map[string][]byte{
					"testme": []byte(`{"clientid":"test-clientid","clientsecret":"test-clientsecret","connectivity_service":"{\"CAs_path\":\"test-CAs_path\",\"CAs_signing_path\":\"test-CAS_signing_path\",\"api_path\":\"test-api_path\",\"tunnel_path\":\"test-tunnel_path\",\"url\":\"test-url\"}","credential-type":"test-credential-type","instance_external_name":"test-instance_external_name","instance_guid":"test-instance_guid","instance_name":"test-instance_name","label":"test-label","plan":"test-plan","subaccount_id":"test-subaccount_id","subaccount_subdomain":"test-subaccount_subdomain","tags":"[\"test\",\"me\",\"plz\"]","token_service_domain":"test-token_service_domain","token_service_url":"test-token_service_url","token_service_url_pattern":"test-token_service_pattern","token_service_url_pattern_tenant_key":"test","type":"test-type","xsappname":"test-xsappname"}`),
				},
				secretRootKey: "testme",
			},
			expected: `{"clientid":"test-clientid","clientsecret":"test-clientsecret","connectivity_service":{"CAs_path":"test-CAs_path","CAs_signing_path":"test-CAS_signing_path","api_path":"test-api_path","tunnel_path":"test-tunnel_path","url":"test-url"},"credential-type":"test-credential-type","instance_guid":"test-instance_guid","instance_name":"test-instance_name","label":"test-label","plan":"test-plan","subaccount_id":"test-subaccount_id","subaccount_subdomain":"test-subaccount_subdomain","tags":["test","me","plz"],"token_service_domain":"test-token_service_domain","token_service_url":"test-token_service_url","token_service_url_pattern":"test-token_service_pattern","token_service_url_pattern_tenant_key":"test","type":"test-type","xsappname":"test-xsappname"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := newEncodedSecretSvcKey(tt.args.secretRootKey, &v1.Secret{
				Data: tt.args.data,
			})
			require.NoError(t, err)
			require.Equal(t, tt.expected, actual)
		})
	}
}
