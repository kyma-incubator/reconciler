package keb

//ConfigurationAsMap flattens the list of configuration entities to a map.
//Component struct is generated from OpenAPI.
func (c Component) ConfigurationAsMap() map[string]interface{} {
	result := make(map[string]interface{}, len(c.Configuration))
	for _, cfg := range c.Configuration {
		result[cfg.Key] = cfg.Value
	}
	return result
}
