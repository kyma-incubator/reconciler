package rendering

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"net/http"
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
	return cp.provider.WithFilter(filter)
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

func NewExternalComponentAuthenticator(token string) chart.ExternalComponentAuthenticator {
	return ExternalComponentAuthenticator{
		token: token,
	}
}

type ExternalComponentAuthenticator struct {
	token string
}

func (e ExternalComponentAuthenticator) Do(r *http.Request) error {
	var bearer = "Bearer " + e.token
	r.Header.Add("Authorization", bearer)

	return nil
}
