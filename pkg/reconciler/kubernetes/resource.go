package kubernetes

import (
	"fmt"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"strings"
)

type cache map[string][]*unstructured.Unstructured

func (c cache) GetKind(kind string) []*unstructured.Unstructured {
	return c[strings.ToLower(kind)]
}

func (c cache) Add(u *unstructured.Unstructured) {
	c[strings.ToLower(u.GetKind())] = append(c[strings.ToLower(u.GetKind())], u)
}

func (c cache) Remove(u *unstructured.Unstructured) {
	kind := strings.ToLower(u.GetKind())
	c[kind] = removeFromSlice(c[kind], u)
}

type Resource struct {
	Kind      string
	Name      string
	Namespace string
}

func (r *Resource) String() string {
	return fmt.Sprintf("KubernetesResource [Kind:%s,Namespace:%s,Name:%s]", r.Kind, r.Namespace, r.Name)
}

type ResourceList struct {
	resources []*unstructured.Unstructured
	cache     cache
}

func NewResourceList(unstructs []*unstructured.Unstructured) *ResourceList {
	cache := make(cache)
	for _, u := range unstructs {
		cache.Add(u)
	}
	return &ResourceList{
		resources: unstructs,
		cache:     cache,
	}
}

func (r *ResourceList) visit(unstructs []*unstructured.Unstructured, callback func(u *unstructured.Unstructured) error) error {
	for _, u := range unstructs {
		if err := callback(u); err != nil {
			return err
		}
	}
	return nil
}

func (r *ResourceList) Visit(callback func(u *unstructured.Unstructured) error) error {
	return r.visit(r.resources, callback)
}

func (r *ResourceList) VisitByKind(kind string, callback func(u *unstructured.Unstructured) error) error {
	return r.visit(r.cache.GetKind(kind), callback)
}

func (r *ResourceList) Get(kind, name, namespace string) *unstructured.Unstructured {
	for _, u := range r.cache.GetKind(kind) {
		if u.GetKind() == kind && u.GetName() == name {
			if u.GetNamespace() == "" || u.GetNamespace() == namespace {
				//`u` is equal if namespace is undefined or defined namespace is equal to provided namespace
				return u
			}
		}
	}
	return nil
}

func (r *ResourceList) GetByKind(kind string) []*unstructured.Unstructured {
	return r.cache.GetKind(kind)
}

func (r *ResourceList) Remove(u *unstructured.Unstructured) {
	r.resources = removeFromSlice(r.resources, u)
	r.cache.Remove(u)
}

func (r *ResourceList) Add(u *unstructured.Unstructured) {
	r.Remove(u) //ensure resource does not exist before adding it
	r.resources = append(r.resources, u)
	r.cache.Add(u)
}

func (r *ResourceList) Len() int {
	return len(r.resources)
}

func removeFromSlice(slc []*unstructured.Unstructured, u *unstructured.Unstructured) []*unstructured.Unstructured {
	for idx, uInSlc := range slc {
		if uInSlc.GetName() == u.GetName() && uInSlc.GetNamespace() == u.GetNamespace() && uInSlc.GetKind() == u.GetKind() {
			return append(slc[:idx], slc[idx+1:]...)
		}
	}
	return slc
}
