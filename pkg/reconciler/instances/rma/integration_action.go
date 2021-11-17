package rma

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/model"
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
	RmiChartURLConfig  = "rmi.chartUrl"
	RmiNamespaceConfig = "rmi.namespace"
)

type IntegrationAction struct {
	name         string
	http         http.Client
	kube         KubeClient
	mux          sync.Mutex
	archives     map[string][]byte
	chartVerExpr *regexp.Regexp
}

func NewIntegrationAction(name string, kubeClient KubeClient) *IntegrationAction {
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
	context.Logger.Debugf("Performing %s action for shoot %s", a.name, context.Task.Metadata.ShootName)

	chartURL := getConfigString(context.Task.Configuration, RmiChartURLConfig)
	if chartURL == "" {
		err := fmt.Errorf("missing required configuration: %s", RmiChartURLConfig)
		context.Logger.Error(err)
		return err
	}
	namespace := getConfigString(context.Task.Configuration, RmiNamespaceConfig)
	if namespace == "" {
		err := fmt.Errorf("missing required configuration: %s", RmiNamespaceConfig)
		context.Logger.Error(err)
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
			return a.install(context, cfg, chartURL, releaseName, namespace)
		}

		// If the release exists, only run helm upgrade if the integration chart version is different.
		// This is necessary to avoid overloading of the control plane K8S API as reconciliation for all runtimes are scheduled periodically.
		// Proceed also with the upgrade if any of the chart versions cannot reliably be determined
		upgradeVersion := a.getChartVersionFromURL(chartURL)
		releaseVersion := ""
		if helmRelease.Chart != nil && helmRelease.Chart.Metadata != nil {
			releaseVersion = helmRelease.Chart.Metadata.Version
		}
		switch {
		case upgradeVersion == "" || releaseVersion == "":
			context.Logger.Warnf("cannot reliably determine monitoring integration chart versions (release/upgrade: %s/%s). Proceeding with rmi upgrade...", releaseVersion, upgradeVersion)
		case upgradeVersion == releaseVersion && helmRelease.Info.Status == release.StatusDeployed:
			context.Logger.Debugf("%s-%s target version matches release version, skipping upgrade.", RmiChartName, releaseName)
			return nil
		default:
			context.Logger.Infof("%s-%s target version: %s release version/status: %s/%s, starting upgrade.", RmiChartName, releaseName, upgradeVersion, releaseVersion, helmRelease.Info.Status)
		}

		return a.upgrade(context, cfg, chartURL, releaseName, namespace)
	case model.OperationTypeDelete:
		if err == nil {
			return a.delete(cfg, releaseName)
		}
	}

	return nil
}

func (a *IntegrationAction) install(context *service.ActionContext, cfg *action.Configuration, chartURL, releaseName, namespace string) error {
	installAction := action.NewInstall(cfg)
	installAction.ReleaseName = releaseName
	installAction.Namespace = namespace
	installAction.Timeout = 6 * time.Minute
	installAction.Wait = true
	installAction.Atomic = true
	chart, err := a.fetchChart(context.Context, chartURL)
	if err != nil {
		return errors.Wrapf(err, "while fetching rmi chart from %s", chartURL)
	}
	username := context.Task.Metadata.InstanceID
	password, err := generatePassword(16)
	if err != nil {
		return errors.Wrap(err, "while generating new auth password")
	}
	overrides := generateOverrideMap(context, username, password)

	_, err = installAction.Run(chart, overrides)
	if err != nil {
		return errors.WithMessagef(err, "helm install %s-%s failed", RmiChartName, releaseName)
	}

	setAuthCredetialOverrides(context.Task.Configuration, username, password)
	return nil
}

func (a *IntegrationAction) upgrade(context *service.ActionContext, cfg *action.Configuration, chartURL, releaseName, namespace string) error {

	upgradeAction := action.NewUpgrade(cfg)
	upgradeAction.Namespace = namespace
	upgradeAction.Timeout = 5 * time.Minute
	upgradeAction.Wait = true
	upgradeAction.Atomic = true
	upgradeAction.MaxHistory = RmiHelmMaxHistory
	chart, err := a.fetchChart(context.Context, chartURL)
	if err != nil {
		return errors.Wrapf(err, "while fetching rmi chart from %s", chartURL)
	}

	username := context.Task.Metadata.InstanceID
	password, err := a.fetchPasswordFromAuthSecret(context.Context, releaseName, namespace)
	if err != nil {
		return errors.WithMessage(err, "failed to fetch auth credentials from secret")
	}
	overrides := generateOverrideMap(context, username, password)

	_, err = upgradeAction.Run(releaseName, chart, overrides)
	if err != nil {
		return errors.WithMessagef(err, "helm upgrade %s-%s failed", RmiChartName, releaseName)
	}

	setAuthCredetialOverrides(context.Task.Configuration, username, password)
	return nil
}

func (a *IntegrationAction) delete(cfg *action.Configuration, releaseName string) error {
	uninstallAction := action.NewUninstall(cfg)
	uninstallAction.Timeout = 5 * time.Minute

	_, err := uninstallAction.Run(releaseName)
	if err != nil {
		return errors.WithMessagef(err, "helm delete %s-%s failed", RmiChartName, releaseName)
	}

	return nil
}

func (a *IntegrationAction) newActionConfig(context *service.ActionContext, namespace string) (*action.Configuration, error) {

	cfg := new(action.Configuration)
	getter, err := a.kube.RESTClientGetter()
	if err != nil {
		return cfg, err
	}
	if err := cfg.Init(getter, namespace, RmiHelmDriver, context.Logger.Debugf); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func (a *IntegrationAction) fetchChart(ctx context.Context, chartURL string) (*chart.Chart, error) {
	a.mux.Lock()
	defer a.mux.Unlock()

	archive := a.archives[chartURL]
	if archive == nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, chartURL, nil)
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

		a.archives[chartURL] = archive
	}

	chart, err := loader.LoadArchive(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}

	return chart, nil
}

func (a *IntegrationAction) fetchPasswordFromAuthSecret(ctx context.Context, release, namespace string) (string, error) {
	client, err := a.kube.ClientSet()
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

func (a *IntegrationAction) getChartVersionFromURL(chartURL string) string {
	match := a.chartVerExpr.FindStringSubmatch(chartURL)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func generatePassword(n int) (string, error) {
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	ret := make([]byte, n)
	for i := 0; i < n; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			return "", err
		}
		ret[i] = letters[num.Int64()]
	}

	return string(ret), nil
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
