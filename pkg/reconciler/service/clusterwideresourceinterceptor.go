package service

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type clusterWideResource struct {
	kind       string
	apiVersion string
}

type ClusterWideResourceInterceptor struct {
	clusterWideResources []clusterWideResource
}

func (c *ClusterWideResourceInterceptor) Intercept(resources *kubernetes.ResourceList, namespace string) error {
	interceptorFunc := func(u *unstructured.Unstructured) error {
		//clean namespace field from cluster-wide resource template
		u.SetNamespace("")
		return nil
	}

	for i := range c.clusterWideResources {
		err := resources.VisitByKindAndAPIVersion(c.clusterWideResources[i].kind, c.clusterWideResources[i].apiVersion, interceptorFunc)
		if err != nil {
			return err
		}
	}

	return nil
}

func newClusterWideResourceInterceptor() *ClusterWideResourceInterceptor {
	return &ClusterWideResourceInterceptor{
		clusterWideResources: []clusterWideResource{
			{
				kind:       "clusterrolebinding",
				apiVersion: "rbac.authorization.k8s.io/v1",
			},
			{
				kind:       "clusterrole",
				apiVersion: "rbac.authorization.k8s.io/v1",
			},
			{
				kind:       "mutatingwebhookconfiguration",
				apiVersion: "admissionregistration.k8s.io/v1",
			},
			{
				kind:       "validatingwebhookconfiguration",
				apiVersion: "admissionregistration.k8s.io/v1",
			},
			{
				kind:       "podsecuritypolicy",
				apiVersion: "policy/v1beta1",
			},
		},
	}
}
