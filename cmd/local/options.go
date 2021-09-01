package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/kyma-incubator/reconciler/internal/cli"
	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/pkg/errors"

	//Register all reconcilers
	_ "github.com/kyma-incubator/reconciler/pkg/reconciler/instances"
)

type Options struct {
	*cli.Options
	kubeconfigFile string
	kubeconfig     string
	version        string
	profile        string
	components     []string
}

func defaultComponentList() []keb.Components {
	defaultComponents := []string{"cluster-essentials", "istio-configuration@istio-system",
		"certificates@istio-system", "logging", "tracing", "kiali", "monitoring", "eventing", "ory", "api-gateway",
		"service-catalog", "service-catalog-addons", "rafter", "helm-broker", "cluster-users", "serverless",
		"application-connector@kyma-integration"}
	return componentsFromStrings(defaultComponents)

}

func NewOptions(o *cli.Options) *Options {
	return &Options{o,
		"",         // kubeconfigFile
		"",         // kubeconfig
		"",         // version
		"",         // profile
		[]string{}, // components
	}
}
func (o *Options) Kubeconfig() string {
	return o.kubeconfig
}

func componentsFromStrings(list []string) []keb.Components {
	var components []keb.Components
	for _, item := range list {
		s := strings.Split(item, "@")
		name := s[0]
		namespace := "kyma-system"
		if len(s) >= 2 {
			namespace = s[1]
		}
		components = append(components, keb.Components{Component: name, Namespace: namespace})
	}
	return components
}

func (o *Options) Components() []keb.Components {
	if len(o.components) > 0 {
		return componentsFromStrings(o.components)
	}
	return defaultComponentList()
}

func (o *Options) Validate() error {
	err := o.Options.Validate()
	if err != nil {
		return err
	}
	if o.kubeconfigFile == "" {
		envKubeconfig, ok := os.LookupEnv("KUBECONFIG")
		if !ok {
			return fmt.Errorf("KUBECONFIG environment variable and kubeconfig flag is missing")
		}
		o.kubeconfigFile = envKubeconfig
	}
	if !file.Exists(o.kubeconfigFile) {
		return fmt.Errorf("Reference kubeconfig file '%s' not found", o.kubeconfigFile)
	}
	content, err := ioutil.ReadFile(o.kubeconfigFile)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to read kubeconfig file '%s'", o.kubeconfigFile))
	}
	o.kubeconfig = string(content)
	return nil
}
