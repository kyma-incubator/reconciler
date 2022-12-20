package chart

import (
	"net/http"
	"strings"

	"github.com/imdario/mergo"
)

//go:generate mockery --name=ExternalComponentAuthenticator --outpkg=mocks --case=underscore
type ExternalComponentAuthenticator interface {
	Do(r *http.Request)
}

type Component struct {
	url                            string
	version                        string
	name                           string
	profile                        string
	namespace                      string
	configuration                  map[string]interface{}
	externalComponentAuthenticator ExternalComponentAuthenticator
}

func (c *Component) isExternalComponent() bool {
	return c.url != ""
}

func (c *Component) isExternalGitComponent() bool {
	return strings.HasSuffix(c.url, ".git")
}

func (c *Component) Configuration() (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for key, value := range c.configuration {
		if err := mergo.Merge(&result, c.convertToNestedMap(key, value), mergo.WithOverride); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (c *Component) SetExternalComponentAuthentication(authenticator ExternalComponentAuthenticator) {
	c.externalComponentAuthenticator = authenticator
}

func (c *Component) ExternalComponentAuthentication() ExternalComponentAuthenticator {
	return c.externalComponentAuthenticator
}

// convertToNestedMap converts a key with dot-notation into a nested map (e.g. a.b.c=value become [a:[b:[c:value]]])
func (c *Component) convertToNestedMap(key string, value interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	tokens := strings.Split(key, ".")
	lastNestedMap := result
	for depth, token := range tokens {
		switch depth {
		case len(tokens) - 1: //last token reached, stop nesting
			lastNestedMap[token] = value
		default:
			lastNestedMap[token] = make(map[string]interface{})
			lastNestedMap = lastNestedMap[token].(map[string]interface{})
		}
	}
	return result
}

type ComponentBuilder struct {
	component *Component
}

func NewComponentBuilder(version, name string) *ComponentBuilder {
	return &ComponentBuilder{
		&Component{
			version:       version,
			name:          name,
			configuration: make(map[string]interface{}),
		},
	}
}

func (cb *ComponentBuilder) WithProfile(profile string) *ComponentBuilder {
	cb.component.profile = profile
	return cb
}

func (cb *ComponentBuilder) WithNamespace(namespace string) *ComponentBuilder {
	cb.component.namespace = namespace
	return cb
}

func (cb *ComponentBuilder) WithConfiguration(config map[string]interface{}) *ComponentBuilder {
	cb.component.configuration = config
	return cb
}

func (cb *ComponentBuilder) WithURL(url string) *ComponentBuilder {
	cb.component.url = url
	return cb
}

func (cb *ComponentBuilder) Build() *Component {
	return cb.component
}
