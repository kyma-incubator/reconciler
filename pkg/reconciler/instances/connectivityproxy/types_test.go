package connectivityproxy

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_encodedString_UnmarshalJSON(t *testing.T) {
	type args struct {
		src []byte
	}
	tests := []struct {
		name     string
		args     args
		expected string
	}{
		{
			name: "non empty data",
			args: args{
				src: []byte("dGVzdC1tZS1wbHo="),
			},
			expected: "test-me-plz",
		},
		{
			name: "empty data",
			args: args{
				src: []byte(""),
			},
			expected: "",
		},
		{
			name: "data containing quotes",
			args: args{
				src: []byte(`"InRlc3QtbWUtcGx6Ig=="`),
			},
			expected: `"test-me-plz"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s encodedString
			err := s.UnmarshalJSON(tt.args.src)
			require.NoError(t, err)
			require.Equal(t, tt.expected, string(s))
		})
	}
}

func Test_encodedString_errors_UnmarshalJSON(t *testing.T) {
	var s encodedString
	err := s.UnmarshalJSON([]byte("this-is-not-encoded"))
	require.Error(t, err)
}

func Test_encodedSlice_UnmarshalJSON(t *testing.T) {
	type args struct {
		src []byte
	}
	tests := []struct {
		name     string
		args     args
		expected encodedSlice
	}{
		{
			name: "unmarshal",
			args: args{
				src: []byte("WyJ0ZXN0IiwgIm1lIl0="),
			},
			expected: encodedSlice([]encodedString{
				"test",
				"me",
			}),
		},
		{
			name: "unmarshal empty array",
			args: args{
				src: []byte("W10="),
			},
			expected: encodedSlice([]encodedString{}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s encodedSlice
			err := s.UnmarshalJSON(tt.args.src)
			require.NoError(t, err)
			require.Equal(t, tt.expected, s)
		})
	}
}

func Test_encodedSlice_errors_UnmarshalJSON(t *testing.T) {
	var s encodedSlice
	err := s.UnmarshalJSON([]byte("this-is-not-encoded"))
	require.Error(t, err)
}

func Test_connectivitySvc_UnmarshalJSON(t *testing.T) {
	testData := []byte("ewogICJDQXNfcGF0aCI6ICJ0ZXN0LWNhc19wYXRoIiwKICAiQ0FzX3NpZ25pbmdfcGF0aCI6ICJ0ZXN0LWNhc19zaWduaW5nX3BhdGgiLAogICJhcGlfcGF0aCI6ICJ0ZXN0LWFwaV9wYXRoIiwKICAidHVubmVsX3BhdGgiOiAidGVzdC10dW5uZWxfcGF0aCIsCiAgInVybCI6ICJ0ZXN0LXVybCIKfQo=")

	var s encodedConSvc

	err := s.UnmarshalJSON(testData)
	require.NoError(t, err)
	require.Equal(t, "test-url", s.URL)
	require.Equal(t, "test-cas_path", s.CasPath)
	require.Equal(t, "test-cas_signing_path", s.CasSigningPath)
	require.Equal(t, "test-tunnel_path", s.TunnelPath)
	require.Equal(t, "test-api_path", s.APIPath)
}

func Test_connectivitySvc_errors_UnmarshalJSON(t *testing.T) {
	var s encodedConSvc
	err := s.UnmarshalJSON([]byte("this-is-not-encoded"))
	require.Error(t, err)
}

func Test_svcKey_fromSecret(t *testing.T) {
	data := map[string][]byte{
		"clientid":                             []byte("test_clientid"),
		"clientsecret":                         []byte("test_clientsecret"),
		"connectivity_service":                 []byte(`{"CAs_path":"test-cas_path","CAs_signing_path":"test-cas_signing_path","api_path":"test-api_path","tunnel_path":"test-tunnel_path","url":"test-url"}`),
		"instance_guid":                        []byte("test_instance_guid"),
		"instance_name":                        []byte("test_instance_name"),
		"label":                                []byte("test_label"),
		"plan":                                 []byte("test_plan"),
		"subaccount_id":                        []byte("test_subaccount_id"),
		"subaccount_subdomain":                 []byte("test_subaccount_subdomain"),
		"tags":                                 []byte(`["test","me"]`),
		"token_service_domain":                 []byte("test_service_domain"),
		"token_service_url":                    []byte("test_token_service_url"),
		"token_service_url_pattern":            []byte("test_token_service_url_pattern"),
		"token_service_url_pattern_tenant_key": []byte("test_token_service_url_pattern_tenant_key"),
		"type":                                 []byte("test_type"),
		"xsappname":                            []byte("test_xsappname"),
	}

	var s svcKey
	err := s.fromSecret(&v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Data:       data,
	})
	require.NoError(t, err)

	require.Equal(t, string(data["clientid"]), string(s.ClientID))
	require.Equal(t, string(data["clientsecret"]), string(s.ClientSecret))

	conSvc, err := toConSvc(data["connectivity_service"])
	require.NoError(t, err)
	require.Equal(t, s.ConnectovitySvc, conSvc)

	require.Equal(t, string(data["credential-type"]), string(s.CredentialsType))
	require.Equal(t, string(data["instance_name"]), string(s.InstanceName))
	require.Equal(t, string(data["instance_guid"]), string(s.InstanceGUID))
	require.Equal(t, string(data["label"]), string(s.Label))
	require.Equal(t, string(data["plan"]), string(s.Plan))
	require.Equal(t, string(data["subaccount_id"]), string(s.SubaccountID))
	require.Equal(t, string(data["subaccount_subdomain"]), string(s.SubaccountSubdomain))

	tags, err := toTags(data["tags"])
	require.NoError(t, err)
	require.Equal(t, tags, s.Tags)

	require.Equal(t, string(data["token_service_domain"]), string(s.TokenSvcDomain))
	require.Equal(t, string(data["token_service_url"]), string(s.TokenSvcURL))
	require.Equal(t, string(data["token_service_url_pattern"]), string(s.TokenSvcURLPattern))
	require.Equal(t, string(data["token_service_url_pattern_tenant_key"]), string(s.TokenSVCURLPatternTenantKey))
	require.Equal(t, string(data["type"]), string(s.Type))
	require.Equal(t, string(data["xsappname"]), string(s.XsAppName))
}

func toConSvc(data []byte) (encodedConSvc, error) {
	type conSvc encodedConSvc

	var tmp conSvc
	if err := json.Unmarshal(data, &tmp); err != nil {
		return encodedConSvc{}, err
	}

	return encodedConSvc(tmp), nil
}

func toTags(data []byte) (encodedSlice, error) {
	var tmp []string
	if err := json.Unmarshal(data, &tmp); err != nil {
		return encodedSlice{}, err
	}

	var out encodedSlice
	for _, item := range tmp {
		out = append(out, encodedString(item))
	}

	return out, nil
}
