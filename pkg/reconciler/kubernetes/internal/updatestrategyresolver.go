package internal

import (
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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

var justUpdate = func(*unstructured.Unstructured, *resource.Helper) (UpdateStrategy, error) {
	return PatchUpdateStrategy, nil
}

var skipUpdate = func(*unstructured.Unstructured, *resource.Helper) (UpdateStrategy, error) {
	return SkipUpdateStrategy, nil
}

var updateStatefulset = func(u *unstructured.Unstructured, h *resource.Helper) (UpdateStrategy, error) {

	obj, err := h.Get(u.GetNamespace(), u.GetName())
	if err != nil {
		if errors.IsNotFound(err) {
			return PatchUpdateStrategy, nil
		}
		return "", err
	}

	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return "", err
	}

	if unstructuredObj == nil {
		return PatchUpdateStrategy, nil
	}

	var sfs appsv1.StatefulSet
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj, &sfs); err != nil {
		return "", err
	}

	if len(sfs.Spec.VolumeClaimTemplates) > 0 { //do not replace STS which have a PVC inside
		return PatchUpdateStrategy, nil
	}

	return ReplaceUpdateStrategy, nil
}

func newDefaultUpdateStrategyResolver(helper *resource.Helper) UpdateStrategyResolver {
	return &DefaultUpdateStrategyResolver{
		helper: helper,
		typeToStrategyMapping: map[string]startegyFn{
			"pod":                   skipUpdate,
			"job":                   skipUpdate,
			"persistentvolumeclaim": justUpdate,
			"serviceaccount":        justUpdate,
			"statefulset":           updateStatefulset,
		},
	}
}

type DefaultUpdateStrategyResolver struct {
	helper *resource.Helper

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
