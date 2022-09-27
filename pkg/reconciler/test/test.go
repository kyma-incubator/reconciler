package test

// NewGlobalComponentConfiguration returns default configuration values required by several Kyma components.
// Deprecated: Remove this fct after all Kyma components are working without global configurations.
// nolint:SA1019 - don't report this deprecated function during linting
func NewGlobalComponentConfiguration() map[string]interface{} {
	return map[string]interface{}{
		"global.ingress.domainName": "local.kyma.dev",
	}
}
