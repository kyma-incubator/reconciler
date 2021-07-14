package chart

type ComponentSet struct {
	kubeconfig string
	version    string
	profile    string
	components []*Component
}

func NewComponentSet(kubeconfig, version, profile string, components []*Component) *ComponentSet {
	return &ComponentSet{
		kubeconfig: kubeconfig,
		version:    version,
		profile:    profile,
		components: components,
	}
}

type Component struct {
	name          string
	namespace     string
	configuration map[string]interface{}
}

func NewComponent(name, namespace string, configuration map[string]interface{}) *Component {
	return &Component{
		name:          name,
		namespace:     namespace,
		configuration: configuration,
	}
}
