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
	values         []string
}

func defaultComponents() []string {
	return []string{"cluster-essentials", "istio-configuration@istio-system",
		"certificates@istio-system", "logging", "tracing", "kiali", "monitoring", "eventing", "ory", "api-gateway",
		"service-catalog", "service-catalog-addons", "rafter", "helm-broker", "cluster-users", "serverless",
		"application-connector@kyma-integration"}
}
func defaultValues() []string {
	return []string{
		"tracing.authProxy.config.useDex=false",
		"tracing.authProxy.configDocsLink=https://kyma-project.io/docs",
		"kiali.authProxy.config.useDex=false",
		"kiali.authProxy.configDocsLink=https://kyma-project.io/docs",
	}
}

func NewOptions(o *cli.Options) *Options {
	return &Options{o,
		"",         // kubeconfigFile
		"",         // kubeconfig
		"",         // version
		"",         // profile
		[]string{}, // components
		[]string{}, // values
	}
}
func (o *Options) Kubeconfig() string {
	return o.kubeconfig
}

func componentsFromStrings(list []string, values []string) []keb.Components {
	var components []keb.Components
	for _, item := range list {
		s := strings.Split(item, "@")
		name := s[0]
		namespace := "kyma-system"
		if len(s) >= 2 {
			namespace = s[1]
		}
		configuration := []keb.Configuration{}
		for _, keyValue := range values {
			splitKeyValue := strings.Split(keyValue, "=")
			splitKey := strings.Split(splitKeyValue[0], ".")
			keyComponent := splitKey[0]
			if keyComponent == name {
				configuration = append(configuration, keb.Configuration{Key: strings.Join(splitKey[1:], "."), Value: splitKeyValue[1]})
				//configuration = append(configuration, keb.Configuration{Key: splitKeyValue[0], Value: splitKeyValue[1]})
			}
			if keyComponent == "global" {
				configuration = append(configuration, keb.Configuration{Key: splitKeyValue[0], Value: splitKeyValue[1]})
			}
		}
		components = append(components, keb.Components{Component: name, Namespace: namespace, Configuration: configuration})
	}
	return components
}

func (o *Options) Components() []keb.Components {
	components := o.components
	if len(components) == 0 {
		components = defaultComponents()
	}
	values := append(defaultValues(), o.values...)
	return componentsFromStrings(components, values)
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
