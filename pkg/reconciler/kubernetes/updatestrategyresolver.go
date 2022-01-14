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

func (d *DefaultUpdateStrategyResolver) Resolve(_ *resource.Info) (UpdateStrategy, error) {
	return PatchUpdateStrategy, nil
}
