package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/kyma-incubator/reconciler/internal/cli"
	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/magiconair/properties"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
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

type KymaComponentList struct {
	DefaultNamespace string `yaml:"defaultNamespace" json:"defaultNamespace"`
	Prerequisites    []Component
	Components       []Component
}
type Component struct {
	Name      string
	Namespace string
}

func componentsFromFile(path string) ([]string, error) {
	componentsFile, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Can't read components file %s", path)
	}
	data := &KymaComponentList{}
	err = yaml.Unmarshal(componentsFile, &data)
	var defaultComponents []string

	for _, c := range data.Components {
		if c.Namespace != "" {
			defaultComponents = append(defaultComponents, c.Name+"@"+c.Namespace)
		} else {
			defaultComponents = append(defaultComponents, c.Name)
		}
	}
	return defaultComponents, nil
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
		for _, value := range values {
			props, err := properties.LoadString(value)

			if err != nil {
				panic(fmt.Errorf("Can't parse value %s", value))
			}
			key := props.Keys()[0]
			splitKey := strings.Split(key, ".")
			keyComponent := splitKey[0]
			if keyComponent == name {
				configuration = append(configuration, keb.Configuration{Key: strings.Join(splitKey[1:], "."), Value: props.GetString(key, "")})
			}
			if keyComponent == "global" {
				configuration = append(configuration, keb.Configuration{Key: key, Value: props.GetString(key, "")})
			}
		}
		components = append(components, keb.Components{Component: name, Namespace: namespace, Configuration: configuration})
	}
	return components
}

func (o *Options) Components(defaultComponentsFile string) []keb.Components {

	components := o.components
	if len(o.components) == 0 {
		cFile := o.componentsFile
		if cFile == "" {
			cFile = defaultComponentsFile
		}
		var err error
		components, err = componentsFromFile(cFile)
		if err != nil {
			panic(err)
		}
	}
	return componentsFromStrings(components, o.values)
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
	if len(o.components) > 0 && o.componentsFile != "" {
		return fmt.Errorf("Use one of 'components' or 'component-file' flag")
	}
	return nil
}
