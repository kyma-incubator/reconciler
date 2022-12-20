package rendering

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)

func TestNewProviderWithAuthentication(t *testing.T) {
	t.Run("must add authenticator to Component", func(t *testing.T) {
		// given
		externalComponentAuthenticatorMock := &mocks.ExternalComponentAuthenticator{}

		chartProviderMock := &mocks.Provider{}

		matcher := func(component *chart.Component) bool {
			return component.ExternalComponentAuthentication() == externalComponentAuthenticatorMock
		}
		chartProviderMock.On("RenderManifest", mock.MatchedBy(matcher)).Return(nil, nil)

		chartProvider := NewProviderWithAuthentication(chartProviderMock, externalComponentAuthenticatorMock)

		// when
		builder := chart.NewComponentBuilder("1.0.0", "test")

		_, err := chartProvider.RenderManifest(builder.Build())
		require.NoError(t, err)

		// then
		chartProviderMock.AssertExpectations(t)
	})
}

func TestExternalComponentAuthenticator_DoHttp(t *testing.T) {
	t.Run("must add Authorization header", func(t *testing.T) {
		// given
		token := "token"
		authenticator := NewExternalComponentAuthenticator(token)

		// when
		req, err := http.NewRequest("GET", "www.example.com", nil)
		require.NoError(t, err)

		err = authenticator.Do(req)

		// then
		require.NoError(t, err)
		authorizationHeader, ok := req.Header["Authorization"]
		require.True(t, ok)
		require.Equal(t, 1, len(authorizationHeader))
		require.Equal(t, fmt.Sprintf("Bearer %s", token), authorizationHeader[0])
	})
}
