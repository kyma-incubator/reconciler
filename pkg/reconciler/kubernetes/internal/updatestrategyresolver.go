package internal

import (
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/resource"
)

const (
	PatchUpdateStrategy   UpdateStrategy = "PATCH"
	ReplaceUpdateStrategy UpdateStrategy = "REPLACE"
	SkipUpdateStrategy    UpdateStrategy = "SKIP"
)

type UpdateStrategy string

type UpdateStrategyResolver interface {
	Resolve(resource *unstructured.Unstructured) (UpdateStrategy, error)
}

func newDefaultUpdateStrategyResolver(helper *resource.Helper) UpdateStrategyResolver {
	return &DefaultUpdateStrategyResolver{
		helper: helper,
	}
}

type DefaultUpdateStrategyResolver struct {
	helper *resource.Helper
}

func (d *DefaultUpdateStrategyResolver) Resolve(_ *unstructured.Unstructured) (UpdateStrategy, error) {
	return PatchUpdateStrategy, nil
}
