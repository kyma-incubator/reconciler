package kubernetes

import (
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

func newDefaultUpdateStrategyResolver(helper *resource.Helper) UpdateStrategyResolver {
	return &DefaultUpdateStrategyResolver{
		helper: helper,
	}
}

type DefaultUpdateStrategyResolver struct {
	helper *resource.Helper
}

func (d *DefaultUpdateStrategyResolver) Resolve(resourceInfo *resource.Info) (UpdateStrategy, error) {

	kind := resourceInfo.Object.GetObjectKind().GroupVersionKind().Kind
	identifier := resourceInfo.Object.GetObjectKind().GroupVersionKind().GroupVersion().Identifier()
	if identifier == "monitoring.coreos.com/v1" {
		if kind == "Prometheus" || kind == "AlertmanagerConfig" || kind == "Alertmanager" ||
			kind == "PodMonitor" || kind == "Probe" || kind == "PrometheusRule" ||
			kind == "ServiceMonitor" || kind == "ThanosRuler" {
			return ReplaceUpdateStrategy, nil
		}
	}
	return PatchUpdateStrategy, nil
}
