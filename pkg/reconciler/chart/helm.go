package chart

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/imdario/mergo"
	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type HelmClient struct {
	chartDir string
	logger   *zap.SugaredLogger
}

func NewHelmClient(chartDir string, logger *zap.SugaredLogger) (*HelmClient, error) {
	if !file.DirExists(chartDir) {
		return nil, fmt.Errorf("HELM chart directory '%s' not found", chartDir)
	}
	return &HelmClient{
		chartDir: chartDir,
		logger:   logger,
	}, nil
}

func (c *HelmClient) Render(component *Component) (string, error) {
	var helmChart *chart.Chart
	var err error
	if component.url != "" && !strings.HasSuffix(component.url, ".git") {
		err = c.downloadComponentChart(component)
		if err != nil {
			return "", err
		}
		helmChart, err = loader.LoadFile(filepath.Join(c.chartDir, fmt.Sprintf("%s.tgz", component.name))) //Loads archieved chart
		if err != nil {
			return "", err
		}
	} else {
		helmChart, err = loader.Load(filepath.Join(c.chartDir, component.name))
		if err != nil {
			return "", err
		}
	}

	config, err := c.mergeChartConfiguration(helmChart, component, false)
	if err != nil {
		return "", err
	}

	tplAction, err := c.newTemplatingAction(component)
	if err != nil {
		return "", err
	}

	helmRelease, err := tplAction.Run(helmChart, config)
	if err != nil || helmRelease == nil {
		return "", errors.Wrap(err, fmt.Sprintf("Failed to render HELM template for component '%s'", component.name))
	}

	return helmRelease.Manifest, nil
}

func (c *HelmClient) downloadComponentChart(component *Component) error {
	req, err := http.NewRequest("GET", component.url, nil)
	if err != nil {
		return err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return errors.Errorf("failed to fetch %s : %s", component.url, resp.Status)
	}

	buf := bytes.NewBuffer(nil)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	destfile := filepath.Join(c.chartDir, fmt.Sprintf("%s.tgz", component.name))
	if err := os.WriteFile(destfile, buf.Bytes(), 0644); err != nil {
		return err
	}

	return nil
}

func (c *HelmClient) newTemplatingAction(component *Component) (*action.Install, error) {
	cfg, err := c.newActionConfig(component.namespace)
	if err != nil {
		return nil, err
	}

	tplAction := action.NewInstall(cfg)
	tplAction.ReleaseName = component.name
	tplAction.Namespace = component.namespace
	tplAction.Atomic = true
	tplAction.Wait = true
	tplAction.CreateNamespace = true
	tplAction.DryRun = true
	tplAction.Replace = true     // Skip the name check
	tplAction.IncludeCRDs = true //include CRDs in the templated output
	tplAction.ClientOnly = true  //if false, it will validate the manifests against the Kubernetes cluster the kubeclient is currently pointing at

	return tplAction, nil
}

func (c *HelmClient) newActionConfig(namespace string) (*action.Configuration, error) {
	clientGetter := genericclioptions.NewConfigFlags(false)
	clientGetter.Namespace = &namespace
	cfg := new(action.Configuration)
	if err := cfg.Init(clientGetter, namespace, "secrets", c.logger.Debugf); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *HelmClient) Configuration(component *Component) (map[string]interface{}, error) {
	helmChart, err := loader.Load(filepath.Join(c.chartDir, component.name))
	if err != nil {
		return nil, err
	}
	return c.mergeChartConfiguration(helmChart, component, true)
}

func (c *HelmClient) mergeChartConfiguration(chart *chart.Chart, component *Component, withValues bool) (map[string]interface{}, error) {
	result, err := c.profileConfiguration(chart, component.profile, withValues)
	if err != nil {
		return nil, err
	}

	componentConfig, err := component.Configuration()
	if err != nil {
		return nil, err
	}

	if err := mergo.Merge(&result, componentConfig, mergo.WithOverride); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to merge profile configuration with component "+
			"configuration for component '%s'", component.name))
	}

	return result, nil
}

func (c *HelmClient) profileConfiguration(ch *chart.Chart, profileName string, withValues bool) (map[string]interface{}, error) {
	var profile *chart.File
	for _, f := range ch.Files {
		if (f.Name == fmt.Sprintf("profile-%s.yaml", profileName)) || (f.Name == fmt.Sprintf("%s.yaml", profileName)) {
			profile = f
			break
		}
	}

	//if no profile file was found, use the values from values.yaml
	if profile == nil {
		return ch.Values, nil
	}

	profileValues, err := chartutil.ReadValues(profile.Data)
	if err != nil {
		return nil, err
	}

	if withValues {
		if err := mergo.Merge(&ch.Values, profileValues.AsMap(), mergo.WithOverride); err != nil {
			return nil, errors.Wrap(err, "failed to merge values.yaml with profile configuration")
		}
		return ch.Values, nil
	}

	//if a profile file was found, use the values from the <profile>.yaml
	return profileValues, nil
}
