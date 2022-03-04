package cmd

import (
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/model"
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
	list := []string{"{api-gateway,api-gatewayNS,https://github.com/kyma-project/customKyma,main}"}
	expected := []*keb.Component{
		{
			Component: "api-gateway", Namespace: "api-gatewayNS", URL: "https://github.com/kyma-project/customKyma", Version: "main",
		},
	}

	cfg, err := componentsFromStrings(list, nil)
	require.NoError(t, err)
	require.EqualValues(t, expected, cfg)
}

func TestSetClusterStateValues(t *testing.T) {
	values := []string{"global.domainName=example.com", "istio.config.defaultDomain=example.com"}
	state := cluster.State{
		Configuration: &model.ClusterConfigurationEntity{
			Components: []*keb.Component{
				{
					Component: "istio",
					Configuration: []keb.Configuration{
						{
							Key:   "install",
							Value: "true",
						},
					},
				},
				{
					Component: "serverless",
				},
			},
		},
	}
	expected := []*keb.Component{
		{
			Component: "istio",
			Configuration: []keb.Configuration{
				{
					Key:   "install",
					Value: "true",
				},
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
					},
				},
			},
		},
		{
			Component: "serverless",
			Configuration: []keb.Configuration{
				{
					Key: "global",
					Value: map[string]interface{}{
						"domainName": "example.com",
					},
				},
			},
		},
	}

	cfg, err := componentsFromClusterState(state, values)
	require.NoError(t, err)
	require.EqualValues(t, expected, cfg)
}
