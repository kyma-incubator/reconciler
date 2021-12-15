package internal

import (
	"strings"

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

type startegyFn = func(*unstructured.Unstructured, *resource.Helper) (UpdateStrategy, error)

var patchStrategy = func(*unstructured.Unstructured, *resource.Helper) (UpdateStrategy, error) {
	return PatchUpdateStrategy, nil
}

var onlyCreateStrategy = func(u *unstructured.Unstructured, h *resource.Helper) (UpdateStrategy, error) {
	if _, err := h.Get(u.GetNamespace(), u.GetName()); err != nil {
		if errors.IsNotFound(err) {
			return PatchUpdateStrategy, nil
		}
		return "", err
	}
	return SkipUpdateStrategy, nil
}

func newDefaultUpdateStrategyResolver(helper *resource.Helper) UpdateStrategyResolver {
	return &DefaultUpdateStrategyResolver{
		helper: helper,
		typeToStrategyMapping: map[string]startegyFn{
			"pod":                   onlyCreateStrategy,
			"job":                   onlyCreateStrategy,
			"persistentvolumeclaim": patchStrategy,
			"serviceaccount":        patchStrategy,
			"role":                  patchStrategy,
			"rolebinding":           patchStrategy,
			"clusterrole":           patchStrategy,
			"clusterrolebinding":    patchStrategy,
		},
	}
}

type DefaultUpdateStrategyResolver struct {
	helper                *resource.Helper
	typeToStrategyMapping map[string]startegyFn
}

func (d *DefaultUpdateStrategyResolver) Resolve(resource *unstructured.Unstructured) (UpdateStrategy, error) {
	rType := strings.ToLower(resource.GetKind())
	fn, ok := d.typeToStrategyMapping[rType]
	if !ok {
		return ReplaceUpdateStrategy, nil
	}
	return fn(resource, d.helper)
}
