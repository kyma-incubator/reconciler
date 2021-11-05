package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/kyma-incubator/reconciler/internal/components"

	"github.com/kyma-incubator/reconciler/internal/cli"
	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/strvals"
)

type Options struct {
	*cli.Options
	kubeconfigFile string
	kubeconfig     string
	version        string
	profile        string
	components     []string
	values         []string
	componentsFile string
}

func NewOptions(o *cli.Options) *Options {
	return &Options{o,
		"",         // kubeconfigFile
		"",         // kubeconfig
		"",         // version
		"",         // profile
		[]string{}, // components
		[]string{}, // values
		"",         // componentsFile
	}
}
func (o *Options) Kubeconfig() string {
	return o.kubeconfig
}

func componentsFromFile(path string) ([]string, []string, error) {
	var preComps []string
	var defaultComps []string

	compList, err := components.NewComponentList(path)
	if err != nil {
		return preComps, defaultComps, err
	}

	for _, c := range compList.Prerequisites {
		preComps = append(preComps, c.Name)
		defaultComps = append(defaultComps, fmt.Sprintf("{%s,%s,%s}", c.Name, c.Namespace, c.URL))
	}
	for _, c := range compList.Components {
		defaultComps = append(defaultComps, fmt.Sprintf("{%s,%s,%s}", c.Name, c.Namespace, c.URL))
	}
	return preComps, defaultComps, nil
}

func componentsFromStrings(list []string, values []string) ([]*keb.Component, error) {
	var comps []*keb.Component

	vals := map[string]interface{}{}
	for _, value := range values {
		err := strvals.ParseInto(value, vals)
		if err != nil {
			return nil, fmt.Errorf("can't parse value %s", value)
		}
	}

	for _, item := range list {
		if strings.HasPrefix(item, "{") {
			item = item[1 : len(item)-1]
		}
		s := strings.Split(item, ",")
		name := strings.TrimSpace(s[0])
		namespace := components.KymaNamespace
		url := ""
		if len(s) > 1 {
			if strings.TrimSpace(s[1]) != "" {
				namespace = strings.TrimSpace(s[1])
			}
			url = setURLRepository(s[2])
		}
		var configuration []keb.Configuration
		if vals[name] != nil {
			val := vals[name]
			mapValue, ok := val.(map[string]interface{})
			if ok {
				for key, value := range mapValue {
					configuration = append(configuration, keb.Configuration{Key: key, Value: value})

				}
			} else {
				return nil, fmt.Errorf("expected nested values for component %s, got value %s", name, val)
			}
		}

		if vals["global"] != nil {
			configuration = append(configuration, keb.Configuration{Key: "global", Value: vals["global"]})
		}
		comps = append(comps, &keb.Component{URL: url, Component: name, Namespace: namespace, Configuration: configuration})
	}

	return comps, nil
}

func setURLRepository(url string) string {
	// TODO add support for credentials
	return strings.TrimSpace(url)
}

func (o *Options) Components(defaultComponentsFile string) ([]string, []*keb.Component, error) {
	var preComps []string

	comps := o.components
	if len(o.components) == 0 {
		cFile := o.componentsFile
		if cFile == "" {
			cFile = defaultComponentsFile
		}
		var err error
		preComps, comps, err = componentsFromFile(cFile)
		if err != nil {
			return preComps, nil, err
		}
	}

	mergedComps, err := componentsFromStrings(comps, o.values)
	if err != nil {
		return preComps, nil, err
	}

	return preComps, mergedComps, err
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
		return fmt.Errorf("reference kubeconfig file '%s' not found", o.kubeconfigFile)
	}

	content, err := ioutil.ReadFile(o.kubeconfigFile)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to read kubeconfig file '%s'", o.kubeconfigFile))
	}
	o.kubeconfig = string(content)

	if len(o.components) > 0 && o.componentsFile != "" {
		return fmt.Errorf("use one of 'components' or 'component-file' flag")
	}
	return nil
}
