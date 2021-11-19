package preaction

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/log"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"strings"
)

const (
	removeNatsOperatorStepName = "removeNatsOperator"
	natsOperatorLastVersion    = "1.24.7"
	natsSubChartPath           = "eventing/charts/nats"
	eventingNats               = "eventing-nats"
	oldConfigValue             = "global.image.repository"
	newConfigValue             = "eu.gcr.io/kyma-project"
	crdPlural                  = "customresourcedefinitions"
)

var (
	natsOperatorCRDsToDelete = []string{"natsclusters.nats.io", "natsserviceroles.nats.io"}
)

type kubeClientProvider func(context *service.ActionContext, logger *zap.SugaredLogger) (kubernetes.Client, error)

type removeNatsOperatorStep struct {
	kubeClientProvider
}

func newRemoveNatsOperatorStep() *removeNatsOperatorStep {
	return &removeNatsOperatorStep{
		kubeClientProvider: defaultKubeClientProvider,
	}
}

func defaultKubeClientProvider(context *service.ActionContext, logger *zap.SugaredLogger) (kubernetes.Client, error) {
	kubeClient, err := kubernetes.NewKubernetesClient(context.Task.Kubeconfig, logger, &kubernetes.Config{
		ProgressInterval: progressTrackerInterval,
		ProgressTimeout:  progressTrackerTimeout,
	})
	if err != nil {
		return nil, err
	}
	return kubeClient, nil
}

func (r *removeNatsOperatorStep) Execute(context *service.ActionContext, logger *zap.SugaredLogger) error {
	// no kyma 2.x+ clusters contain nats-operator resources
	clusterVersion := 2
	if clusterVersion > 1 {
		logger.With(log.KeyReason, "NATS-operator resources do not exist on clusters with kyma 2.x+ version").Info("Step skipped")
		return nil
	}

	kubeClient, err := r.kubeClientProvider(context, logger)

	if err != nil {
		return err
	}

	// decorate logger
	logger = logger.With(log.KeyStep, removeNatsOperatorStepName)

	err = r.removeNatsOperatorResources(context, kubeClient, logger)
	if err != nil {
		return err
	}

	err = r.removeNatsOperatorCRDs(kubeClient, logger)
	return err
}

func (r *removeNatsOperatorStep) removeNatsOperatorResources(context *service.ActionContext, kubeClient kubernetes.Client, logger *zap.SugaredLogger) error {
	// get charts from the version 1.2.x, where the NATS-operator resources still exist
	comp := GetResourcesFromVersion(natsOperatorLastVersion, natsSubChartPath)

	manifest, err := context.ChartProvider.RenderManifest(comp)
	if err != nil {
		return err
	}

	// set the right eventing name, which went lost after rendering
	manifest.Manifest = strings.ReplaceAll(manifest.Manifest, natsSubChartPath, eventingNats)

	logger.Info("Removing nats-operator chart resources")
	// remove all the existing nats-operator resources, installed via charts
	_, err = kubeClient.Delete(context.Context, manifest.Manifest, namespace)
	return err
}

// delete the leftover CRDs, which were outside of charts
func (r *removeNatsOperatorStep) removeNatsOperatorCRDs(kubeClient kubernetes.Client, logger *zap.SugaredLogger) error {
	logger.Info("Removing nats-operator CRDs")
	for _, crdName := range natsOperatorCRDsToDelete {
		_, err := kubeClient.DeleteResourceByKindAndNameAndNamespace(crdPlural, crdName, namespace)
		if err != nil && !errors.IsNotFound(err) {
			logger.Errorf("Failed to delete the nats-operator CRDs, name='%s', namespace='%s': %s", crdName, namespace, err)
			return err
		}
		if err == nil {
			logger.Debugf("Deleted %s CRD from %s namespace", crdName, namespace)
		}
	}
	return nil
}

func GetResourcesFromVersion(version, chartPath string) *chart.Component {
	return chart.NewComponentBuilder(version, chartPath).
		WithConfiguration(map[string]interface{}{
			// replace the missing global value, as we are rendering on the subchart level
			oldConfigValue: newConfigValue,
		}).
		WithNamespace(namespace).
		Build()
}
