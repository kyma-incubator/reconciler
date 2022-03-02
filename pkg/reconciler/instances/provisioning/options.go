package provisioning

import "fmt"

// so we should have
// APP_GARDENER_PROJECT and
// APP_GARDENER_KUBECONFIG_PATH

type config struct {
	GardenerProject        string `envconfig:"default=gardenerProject"`
	GardenerKubeconfigPath string `envconfig:"default=./dev/kubeconfig.yaml"`
}

func (c *config) String() string {
	return fmt.Sprintf("Gardener project: %s, KubeconfigPath: %s",
		c.GardenerProject, c.GardenerKubeconfigPath)
}
