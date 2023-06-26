package rendering

import (
	"net/http"
	"os"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/pkg/errors"
)

const (
	TokenEnvVariable    = "GIT_CLONE2" //#nosec [-- Ignore nosec false positive. It's not a credential, just an environment variable name]
	AuthorizationHeader = "Authorization"
)

func NewProviderWithAuthentication(provider chart.Provider, authenticator chart.ExternalComponentAuthenticator) chart.Provider {
	return ChartProviderWithAuthentication{
		provider:      provider,
		authenticator: authenticator,
	}
}

type ChartProviderWithAuthentication struct {
	provider      chart.Provider
	authenticator chart.ExternalComponentAuthenticator
}

func (cp ChartProviderWithAuthentication) WithFilter(filter chart.Filter) chart.Provider {
	return &ChartProviderWithAuthentication{
		provider:      cp.provider.WithFilter(filter),
		authenticator: cp.authenticator,
	}
}

func (cp ChartProviderWithAuthentication) RenderCRD(version string) ([]*chart.Manifest, error) {
	return cp.provider.RenderCRD(version)
}

func (cp ChartProviderWithAuthentication) RenderManifest(component *chart.Component) (*chart.Manifest, error) {
	component.SetExternalComponentAuthentication(cp.authenticator)

	return cp.provider.RenderManifest(component)
}

func (cp ChartProviderWithAuthentication) Configuration(component *chart.Component) (map[string]interface{}, error) {
	return cp.provider.Configuration(component)
}

func NewExternalComponentAuthenticator() (chart.ExternalComponentAuthenticator, error) {
	token := os.Getenv(TokenEnvVariable)
	if token == "" {
		return nil, errors.New("failed to get chart download access token")
	}

	return ExternalComponentAuthenticator{
		token: token,
	}, nil
}

type ExternalComponentAuthenticator struct {
	token string
}

func (e ExternalComponentAuthenticator) Do(r *http.Request) {
	var bearer = "Bearer " + e.token
	r.Header.Add(AuthorizationHeader, bearer)
}
