package cmd

import (
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/stretchr/testify/require"
)

func TestOverlappingNestedValues(t *testing.T) {
	list := []string{"api-gateway"}
	values := []string{"global.ingress.domainName=example.com", "global.domainName=example.com", "api-gateway.config.defaultDomain=example.com"}
	expected := []*keb.Component{
		{
			Component: "api-gateway", Namespace: "kyma-system",
			Configuration: []keb.Configuration{
				{
					Key: "config",
					Value: map[string]interface{}{
						"defaultDomain": "example.com",
					},
				},
				{
					Key: "global",
					Value: map[string]interface{}{
						"domainName": "example.com",
						"ingress": map[string]interface{}{
							"domainName": "example.com",
						},
					},
				},
			},
		},
	}

	cfg, err := componentsFromStrings(list, values)
	require.NoError(t, err)
	require.EqualValues(t, expected, cfg)
}

func TestSetCustomNamespaceAndUrl(t *testing.T) {
	list := []string{"{api-gateway,api-gatewayNS,https://github.com/kyma-project/customKyma}"}
	expected := []*keb.Component{
		{
			Component: "api-gateway", Namespace: "api-gatewayNS", URL: "https://github.com/kyma-project/customKyma",
		},
	}

	cfg, err := componentsFromStrings(list, nil)
	require.NoError(t, err)
	require.EqualValues(t, expected, cfg)
}
