package kubernetes

import (
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/cli-runtime/pkg/resource"
)

const (
	PatchUpdateStrategy   UpdateStrategy = "PATCH"
	ReplaceUpdateStrategy UpdateStrategy = "REPLACE"
	SkipUpdateStrategy    UpdateStrategy = "SKIP"
)

type UpdateStrategy string

type UpdateStrategyResolver interface {
	Resolve(resource *resource.Info) (UpdateStrategy, error)
}

func newDefaultUpdateStrategyResolver(helper *resource.Helper, logger *zap.SugaredLogger) UpdateStrategyResolver {
	return &DefaultUpdateStrategyResolver{
		helper: helper,
		logger: logger,
	}
}

type DefaultUpdateStrategyResolver struct {
	helper *resource.Helper
	logger *zap.SugaredLogger
}

func (d *DefaultUpdateStrategyResolver) Resolve(resourceInfo *resource.Info) (UpdateStrategy, error) {
	gvk := resourceInfo.Object.GetObjectKind().GroupVersionKind()
	kind := gvk.Kind //to improve readability
	identifier := gvk.GroupVersion().Identifier()
	if identifier == "monitoring.coreos.com/v1" { //TODO: drop me after #678 is implemented
		if kind == "Prometheus" || kind == "AlertmanagerConfig" || kind == "Alertmanager" ||
			kind == "PodMonitor" || kind == "Probe" || kind == "PrometheusRule" ||
			kind == "ServiceMonitor" || kind == "ThanosRuler" {
			return ReplaceUpdateStrategy, nil
		}
	}
	//don't update jobs after they were created: not allowed in K8s
	//(see https://github.com/helm/helm/issues/7725#issuecomment-617373825)
	if identifier == "batch/v1" && kind == "Job" {
		if !errors.IsNotFound(resourceInfo.Get()) {
			d.logger.Debugf("Job '%s@%s' already exists: update skipped to avoid immuteable fields error",
				resourceInfo.Name, resourceInfo.Namespace)
			return SkipUpdateStrategy, nil
		}
	}
	return PatchUpdateStrategy, nil
}
