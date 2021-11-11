package rma

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/kubeclient"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	RmiHelmDriver      = "secret"
	RmiHelmMaxHistory  = 1
	RmiChartName       = "rmi"
	RmiChartUrlConfig  = "rmi.chartUrl"
	RmiNamespaceConfig = "rmi.namespace"
)

type IntegrationAction struct {
	name         string
	http         http.Client
	kube         *kubeclient.KubeClient
	mux          sync.Mutex
	archives     map[string][]byte
	chartVerExpr *regexp.Regexp
}

func NewIntegrationAction(name string, kubeClient *kubeclient.KubeClient) *IntegrationAction {

	return &IntegrationAction{
		name: name,
		kube: kubeClient,
		http: http.Client{
			Timeout: 20 * time.Second,
		},
		archives:     make(map[string][]byte),
		chartVerExpr: regexp.MustCompile(fmt.Sprintf("%s-([a-zA-Z0-9-.]+)\\.tgz$", RmiChartName)),
	}
}

func (a *IntegrationAction) Run(context *service.ActionContext) error {
	context.Logger.Infof("Performing %s action for shoot %s", a.name, context.Task.Metadata.ShootName)

	chartUrl := getConfigString(context.Task.Configuration, RmiChartUrlConfig)
	if chartUrl == "" {
		err := fmt.Errorf("missing required configuration: %s", RmiChartUrlConfig)
		return err
	}
	namespace := getConfigString(context.Task.Configuration, RmiNamespaceConfig)
	if namespace == "" {
		err := fmt.Errorf("missing required configuration: %s", RmiNamespaceConfig)
		return err
	}
	releaseName := context.Task.Metadata.ShootName

	cfg, err := a.newActionConfig(context, namespace)
	if err != nil {
		return err
	}

	histClient := action.NewHistory(cfg)
	histClient.Max = 1
	releases, err := histClient.Run(releaseName)
	if err != nil && err != driver.ErrReleaseNotFound {
		return errors.Wrapf(err, "while querying rmi helm history for release %s", releaseName)
	}
	helmRelease := findLatestRevision(releases)

	switch context.Task.Type {
	case model.OperationTypeReconcile:
		// If a release does not exist, run helm install
		if err == driver.ErrReleaseNotFound {
			return a.install(context, cfg, chartUrl, releaseName, namespace)
		}

		// If the release exists, only run helm upgrade if the integration chart version is different.
		// This is necessary to avoid overloading of the control plane K8S API as reconciliation for all runtimes are scheduled periodically.
		// Proceed also with the upgrade if any of the chart versions cannot reliably be determined
		upgradeVersion := a.getChartVersionFromURL(chartUrl)
		releaseVersion := ""
		if helmRelease.Chart != nil && helmRelease.Chart.Metadata != nil {
			releaseVersion = helmRelease.Chart.Metadata.Version
		}
		switch {
		case upgradeVersion == "" || releaseVersion == "":
			context.Logger.Warnf("cannot reliably determine monitoring integration chart versions (release/upgrade: %s/%s). Proceeding with rmi upgrade...", releaseVersion, upgradeVersion)
		case upgradeVersion == releaseVersion && helmRelease.Info.Status == release.StatusDeployed:
			context.Logger.Infof("%s-%s target version matches release version, skipping upgrade.", RmiChartName, releaseName)
			return nil
		default:
			context.Logger.Infof("%s-%s target version: %s release version/status: %s/%s, starting upgrade.", RmiChartName, releaseName, upgradeVersion, releaseVersion, helmRelease.Info.Status)
		}

		return a.upgrade(context, cfg, chartUrl, releaseName, namespace)
	case model.OperationTypeDelete:
		if err == nil {
			return a.delete(context, cfg, releaseName)
		}
	}

	return nil
}

func (a *IntegrationAction) install(context *service.ActionContext, cfg *action.Configuration, chartUrl, releaseName, namespace string) error {
	installAction := action.NewInstall(cfg)
	installAction.ReleaseName = releaseName
	installAction.Namespace = namespace
	installAction.Timeout = 6 * time.Minute
	installAction.Wait = true
	installAction.Atomic = true
	chart, err := a.fetchChart(context.Context, chartUrl)
	if err != nil {
		context.Logger.Errorf("failed to fetch RMI chart from %s: %s", chartUrl, err)
		return err
	}
	username := context.Task.Metadata.InstanceID
	password := generatePassword(16)
	overrides := generateOverrideMap(context, username, password)

	_, err = installAction.Run(chart, overrides)
	if err != nil {
		context.Logger.Errorf("helm install %s failed: %s", releaseName, err)
		return err
	}

	setAuthCredetialOverrides(context.Task.Configuration, username, password)
	return nil
}

func (a *IntegrationAction) upgrade(context *service.ActionContext, cfg *action.Configuration, chartUrl, releaseName, namespace string) error {

	upgradeAction := action.NewUpgrade(cfg)
	upgradeAction.Namespace = namespace
	upgradeAction.Timeout = 5 * time.Minute
	upgradeAction.Wait = true
	upgradeAction.Atomic = true
	upgradeAction.MaxHistory = RmiHelmMaxHistory
	chart, err := a.fetchChart(context.Context, chartUrl)
	if err != nil {
		context.Logger.Errorf("failed to fetch RMI chart from %s: %s", chartUrl, err)
		return err
	}

	username := context.Task.Metadata.InstanceID
	password, err := a.fetchPasswordFromAuthSecret(context.Context, releaseName, namespace)
	if err != nil {
		context.Logger.Errorf("failed to fetch auth credentials from secret: %s", err)
		return err
	}
	overrides := generateOverrideMap(context, username, password)

	_, err = upgradeAction.Run(releaseName, chart, overrides)
	if err != nil {
		context.Logger.Errorf("helm upgrade %s failed: %s", releaseName, err)
		return err
	}

	setAuthCredetialOverrides(context.Task.Configuration, username, password)
	return nil
}

func (c *IntegrationAction) delete(context *service.ActionContext, cfg *action.Configuration, releaseName string) error {
	uninstallAction := action.NewUninstall(cfg)
	uninstallAction.Timeout = 5 * time.Minute

	_, err := uninstallAction.Run(releaseName)
	if err != nil {
		context.Logger.Errorf("helm delete %s failed: %s", releaseName, err)
	}

	return err
}

func (a *IntegrationAction) newActionConfig(context *service.ActionContext, namespace string) (*action.Configuration, error) {

	cfg := new(action.Configuration)
	if err := cfg.Init(a.kube.RESTClientGetter(), namespace, RmiHelmDriver, context.Logger.Debugf); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func (a *IntegrationAction) fetchChart(ctx context.Context, chartUrl string) (*chart.Chart, error) {
	a.mux.Lock()
	defer a.mux.Unlock()

	archive := a.archives[chartUrl]
	if archive == nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, chartUrl, nil)
		if err != nil {
			return nil, err
		}
		resp, err := a.http.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		archive, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("http status %s", resp.Status)
		}

		a.archives[chartUrl] = archive
	}

	chart, err := loader.LoadArchive(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}

	return chart, nil
}

func (a *IntegrationAction) fetchPasswordFromAuthSecret(ctx context.Context, release, namespace string) (string, error) {
	client, err := a.kube.GetClientSet()
	if err != nil {
		return "", err
	}
	secret, err := client.CoreV1().Secrets(namespace).Get(ctx, fmt.Sprintf("vmuser-%s-%s", RmiChartName, release), metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	if secret.Data == nil {
		return "", errors.New("secret data is empty")
	}
	password := secret.Data["password"]
	if len(password) == 0 {
		return "", errors.New("missing/empty auth credentials")
	}

	return string(password), nil
}

func (a *IntegrationAction) getChartVersionFromURL(chartUrl string) string {
	match := a.chartVerExpr.FindStringSubmatch(chartUrl)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func generatePassword(length int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	rand.Seed(time.Now().UnixNano())
	bytes := make([]rune, length)
	for i := range bytes {
		bytes[i] = letterRunes[rand.Intn(len(letterRunes))]
	}

	return string(bytes)
}

func generateOverrideMap(context *service.ActionContext, username, password string) map[string]interface{} {
	overrideMap := make(map[string]interface{})
	metadata := context.Task.Metadata
	overrideMap["runtime"] = map[string]string{
		"instanceID":      metadata.InstanceID,
		"globalAccountID": metadata.GlobalAccountID,
		"subaccountID":    metadata.SubAccountID,
		"shootName":       metadata.ShootName,
		"planName":        metadata.ServicePlanName,
		"region":          metadata.Region,
	}
	overrideMap["auth"] = map[string]string{
		"username": username,
		"password": password,
	}

	return overrideMap
}

func getConfigString(config map[string]interface{}, key string) string {
	val, ok := config[key]
	if !ok {
		return ""
	}
	rv, ok := val.(string)
	if !ok {
		return ""
	}

	return rv
}

func setAuthCredetialOverrides(configuration map[string]interface{}, username, password string) {
	configuration["vmuser.username"] = username
	configuration["vmuser.password"] = password
}

func findLatestRevision(releases []*release.Release) *release.Release {
	revision := -1
	var release *release.Release = nil
	for _, r := range releases {
		if r.Version > revision {
			release = r
			revision = r.Version
		}
	}

	return release
}
