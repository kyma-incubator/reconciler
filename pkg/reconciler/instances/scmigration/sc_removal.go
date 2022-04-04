package scmigration

import (
	"fmt"
	"log"
	"time"

	gerr "github.com/pkg/errors"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/scmigration/apis/servicecatalog/v1beta1"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

// It was decided that this reconciler should reuse code from sc-removal application
// This file contains reimplementation of https://github.com/kyma-incubator/sc-removal

type scremoval struct {
	k8sCli client.Client
}

func newSCRemovalClient(kubeConfigContent []byte) (*scremoval, error) {
	kubeconfig, err := clientcmd.NewClientConfigFromBytes(kubeConfigContent)
	if err != nil {
		return nil, err
	}
	restConfig, err := kubeconfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	k8sCli, err := client.New(restConfig, client.Options{
		Scheme: scheme.Scheme,
	})
	if err != nil {
		return nil, err
	}
	err = v1beta1.AddToScheme(scheme.Scheme)
	if err != nil {
		return nil, err
	}
	err = apiextensions.AddToScheme(scheme.Scheme)
	if err != nil {
		return nil, err
	}

	return &scremoval{k8sCli}, nil
}

func (c *scremoval) prepareSBUsForRemoval(ac *service.ActionContext) error {
	namespaces := &v1.NamespaceList{}
	err := c.k8sCli.List(ac.Context, namespaces)
	if err != nil {
		return err
	}

	var errs []error
	for _, ns := range namespaces.Items {
		ul := &unstructured.UnstructuredList{}
		ul.SetGroupVersionKind(schema.GroupVersionKind{
			Kind:    "ServiceBindingUsage",
			Group:   "servicecatalog.kyma-project.io",
			Version: "v1alpha1",
		})
		err = c.k8sCli.List(ac.Context, ul, client.InNamespace(ns.Name))
		if meta.IsNoMatchError(err) {
			ac.Logger.Infof("CRD for ServiceBindingUsage not found, skipping SBU removal")
			continue
		}
		if err != nil {
			errs = append(errs, err)
		} else {
			for i := range ul.Items {
				sbu := ul.Items[i]
				ac.Logger.Infof("Removing owner reference from SBU %s/%s", sbu.GetNamespace(), sbu.GetName())
				sbu.SetOwnerReferences([]metav1.OwnerReference{})
				if err = c.k8sCli.Update(ac.Context, &sbu); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}
	if len(errs) != 0 {
		return gerr.Wrap(errors.NewAggregate(errs), "failed to remove owner references from SBUs")
	}
	return nil
}

func (c *scremoval) removeRelease(releaseName string, ac *service.ActionContext) error {
	done := make(chan bool)
	var errs []error
	go func() {
		hasResourcesToCheck := true
		for hasResourcesToCheck {
			hasResourcesToCheck = false
			for _, r := range resources[releaseName] {
				ro := &unstructured.Unstructured{}
				ro.SetGroupVersionKind(r.GetObjectKind().GroupVersionKind())
				if err := c.k8sCli.Get(ac.Context, types.NamespacedName{Name: r.GetName(), Namespace: r.GetNamespace()}, ro); kerrors.IsNotFound(err) || meta.IsNoMatchError(err) {
					continue
				} else if err != nil {
					hasResourcesToCheck = true
					errs = append(errs, gerr.Wrap(err, "getting resource"))
				} else {
					hasResourcesToCheck = true
					if len(ro.GetFinalizers()) != 0 {
						rodc := ro.DeepCopy()
						rodc.SetGroupVersionKind(r.GetObjectKind().GroupVersionKind())
						rodc.SetFinalizers([]string{})
						if err := c.k8sCli.Update(ac.Context, rodc); err != nil {
							errs = append(errs, gerr.Wrap(err, "failed patching"))
						}
					}
				}
				if err := c.k8sCli.Delete(ac.Context, r); kerrors.IsNotFound(err) || meta.IsNoMatchError(err) {
					continue
				} else if err != nil {
					hasResourcesToCheck = true
					errs = append(errs, gerr.Wrap(err, fmt.Sprintf("%T %v", err, "failed deleting")))
				} else {
					hasResourcesToCheck = true
					ac.Logger.Infof("deleting resource %v %v/%v\n", r.GetObjectKind().GroupVersionKind(), r.GetNamespace(), r.GetName())
				}
			}
		}
		done <- true
	}()

	select {
	case <-done:
		return nil
	case <-time.After(10 * time.Minute):
		return gerr.Wrap(errors.NewAggregate(errs), fmt.Sprintf("deleting %v timed out after 10 minutes, likely finalizers keep getting added", releaseName))
	}
}

func (c *scremoval) prepareForRemoval(ac *service.ActionContext) error {
	namespaces := &v1.NamespaceList{}
	err := c.k8sCli.List(ac.Context, namespaces)
	if err != nil {
		return gerr.Wrap(err, "listing namespaces")
	}

	gvkList := []schema.GroupVersionKind{
		{
			Group:   "servicecatalog.k8s.io",
			Kind:    "ServiceBindingList",
			Version: "v1beta1",
		},
		{
			Group:   "servicecatalog.k8s.io",
			Kind:    "ServiceInstanceList",
			Version: "v1beta1",
		},
		{
			Group:   "servicecatalog.k8s.io",
			Kind:    "ServiceBrokerList",
			Version: "v1beta1",
		},
		{
			Kind:    "UsageKind",
			Group:   "servicecatalog.kyma-project.io",
			Version: "v1alpha1",
		},
		{
			Group:   "servicecatalog.k8s.io",
			Kind:    "ServiceClassList",
			Version: "v1beta1",
		},
		{
			Group:   "servicecatalog.k8s.io",
			Kind:    "ServicePlanList",
			Version: "v1beta1",
		},
	}

	for _, gvk := range gvkList {
		for _, ns := range namespaces.Items {
			err := c.removeFinalizers(gvk, ns.Name, ac)
			if meta.IsNoMatchError(err) {
				ac.Logger.Infof("CRD for GVK %s not found, skipping finalizer removal", gvk)
			} else if err != nil {
				return gerr.Wrap(err, fmt.Sprintf("removing finalizers for %v in %v", gvk, ns.Name))
			}
		}
	}
	clusterGVKList := []schema.GroupVersionKind{
		{
			Group:   "servicecatalog.k8s.io",
			Kind:    "ClusterServiceBrokerList",
			Version: "v1beta1",
		},
		{
			Group:   "servicecatalog.k8s.io",
			Kind:    "ClusterServiceClassList",
			Version: "v1beta1",
		},
		{
			Group:   "servicecatalog.k8s.io",
			Kind:    "ClusterServicePlanList",
			Version: "v1beta1",
		},
	}
	for _, gvk := range clusterGVKList {
		err := c.removeFinalizers(gvk, "", ac)
		if meta.IsNoMatchError(err) {
			ac.Logger.Infof("CRD for GVK %s not found, skipping finalizer removal", gvk)
		} else if err != nil {
			return gerr.Wrap(err, fmt.Sprintf("removing finalizers for %v", gvk))
		}
	}

	log.Println("ServiceBindings secrets owner references")
	var bindings = &v1beta1.ServiceBindingList{}
	err = c.k8sCli.List(ac.Context, bindings, client.InNamespace(""))
	if meta.IsNoMatchError(err) {
		ac.Logger.Infof("CRD for ServiceBinding not found, skipping owner reference secret adjustments")
		return nil
	}
	if err != nil {
		return gerr.Wrap(err, "listing bindings")
	}
	for i := range bindings.Items {
		item := bindings.Items[i]
		ac.Logger.Infof("%s/%s", item.Namespace, item.Name)
		item.Finalizers = []string{}
		err := c.k8sCli.Update(ac.Context, &item)
		if err != nil {
			return gerr.Wrap(err, fmt.Sprintf("updating binding %v/%v", item.Namespace, item.Name))
		}

		// find linked secrets
		var secret = &v1.Secret{}
		err = c.k8sCli.Get(ac.Context, client.ObjectKey{
			Namespace: item.Namespace,
			Name:      item.Spec.SecretName,
		}, secret)
		if secret.Name == "" {
			continue
		}
		if err != nil {
			return gerr.Wrap(err, fmt.Sprintf("getting secret %v/%v", secret.Namespace, secret.Name))
		}

		secret.OwnerReferences = []metav1.OwnerReference{}
		err = c.k8sCli.Update(ac.Context, secret)
		if err != nil {
			return gerr.Wrap(err, fmt.Sprintf("updating secret %v/%v", secret.Namespace, secret.Name))
		}
	}

	return nil
}

func (c *scremoval) removeFinalizers(gvk schema.GroupVersionKind, ns string, ac *service.ActionContext) error {
	ul := &unstructured.UnstructuredList{}
	ul.SetGroupVersionKind(gvk)
	if err := c.k8sCli.List(ac.Context, ul, client.InNamespace(ns)); err != nil {
		return gerr.Wrap(err, fmt.Sprintf("listing resources %v", gvk))
	}

	var errs []error
	for i := range ul.Items {
		obj := ul.Items[i]
		obj.SetFinalizers([]string{})
		if err := c.k8sCli.Update(ac.Context, &obj); err != nil {
			errs = append(errs, fmt.Errorf("updating resource %v %v/%v", gvk, obj.GetNamespace(), obj.GetName()))
		}
		ac.Logger.Infof("%s %s/%s: finalizers removed", gvk.Kind, ns, obj.GetName())
	}

	if len(errs) != 0 {
		return gerr.Wrap(errors.NewAggregate(errs), "failed to remove finalizers")
	}
	return nil
}

func (c *scremoval) removeResources(ac *service.ActionContext) error {
	gvkList := []schema.GroupVersionKind{
		{
			Kind:    "ServiceBindingUsage",
			Group:   "servicecatalog.kyma-project.io",
			Version: "v1alpha1",
		},
		{
			Kind:    "UsageKind",
			Group:   "servicecatalog.kyma-project.io",
			Version: "v1alpha1",
		},
		{
			Kind:    "ServiceBinding",
			Group:   "servicecatalog.k8s.io",
			Version: "v1beta1",
		},
		{
			Kind:    "ServiceInstance",
			Group:   "servicecatalog.k8s.io",
			Version: "v1beta1",
		},
		{
			Kind:    "ServiceBroker",
			Group:   "servicecatalog.k8s.io",
			Version: "v1beta1",
		},
		{
			Kind:    "ServiceClass",
			Group:   "servicecatalog.k8s.io",
			Version: "v1beta1",
		},
		{
			Kind:    "ServicePlan",
			Group:   "servicecatalog.k8s.io",
			Version: "v1beta1",
		},
		{
			Kind:    "AddonsConfiguration",
			Group:   "addons.kyma-project.io",
			Version: "v1alpha1",
		},
	}

	namespaces := &v1.NamespaceList{}
	err := c.k8sCli.List(ac.Context, namespaces)
	if err != nil {
		return err
	}

	var errs []error
	for _, gvk := range gvkList {
		for _, namespace := range namespaces.Items {
			ac.Logger.Infof("deleting %s in %s\n", gvk.Kind, namespace.Name)
			u := &unstructured.Unstructured{}
			u.SetGroupVersionKind(gvk)
			err = c.k8sCli.DeleteAllOf(ac.Context, u, client.InNamespace(namespace.Name))
			if meta.IsNoMatchError(err) {
				ac.Logger.Infof("CRD for GVK %s not found, skipping resource deletion", gvk)
				break
			} else if err != nil {
				errs = append(errs, gerr.Wrap(err, fmt.Sprintf("deleting all resources %v in %v", gvk, namespace.Name)))
			}
		}
	}

	clusterGVKList := []schema.GroupVersionKind{
		{
			Kind:    "ClusterAddonsConfiguration",
			Group:   "addons.kyma-project.io",
			Version: "v1alpha1",
		},
		{
			Kind:    "ClusterServiceBroker",
			Group:   "servicecatalog.k8s.io",
			Version: "v1beta1",
		},
		{
			Kind:    "ClusterServiceClass",
			Group:   "servicecatalog.k8s.io",
			Version: "v1beta1",
		},
		{
			Kind:    "ClusterServicePlan",
			Group:   "servicecatalog.k8s.io",
			Version: "v1beta1",
		},
	}
	for _, gvk := range clusterGVKList {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(gvk)
		err = c.k8sCli.DeleteAllOf(ac.Context, u, client.InNamespace(""))
		if meta.IsNoMatchError(err) {
			ac.Logger.Infof("CRD for GVK %s not found, skipping resource deletion", gvk)
			continue
		} else if err != nil {
			errs = append(errs, gerr.Wrap(err, fmt.Sprintf("deleting all resources %v", gvk)))
		}
	}
	return errors.NewAggregate(errs)
}

func (c *scremoval) waitForPodsGone(ac *service.ActionContext, dep unstructured.Unstructured) error {
	path := []string{"spec", "selector", "matchLabels"}
	ls, found, err := unstructured.NestedStringMap(dep.Object, path...)
	if err != nil {
		msg := fmt.Sprintf("unstructured dep %v/%v failed to find selector %v: %v", dep.GetNamespace(), dep.GetName(), path, err)
		return gerr.Wrap(err, msg)
	}
	if !found {
		return fmt.Errorf("unstructured dep %v/%v missing selector %v", dep.GetNamespace(), dep.GetName(), path)
	}
	cnd := func() (bool, error) {
		pods := &v1.PodList{}
		opts := []client.ListOption{client.InNamespace(dep.GetNamespace()), client.MatchingLabels(ls)}
		if err := c.k8sCli.List(ac.Context, pods, opts...); err != nil {
			return false, err
		}
		return len(pods.Items) == 0, nil
	}
	return wait.PollImmediate(5*time.Second, 10*time.Minute, cnd)
}

func (c *scremoval) ensureServiceCatalogNotRunning(ac *service.ActionContext) error {
	done := make(chan error, 2)
	go func() {
		done <- c.waitForPodsGone(ac, serviceCatalogCatalogControllerManagerUnstructuredDeployment)
	}()
	go func() {
		done <- c.waitForPodsGone(ac, helmBrokerUnstructuredDeployment)
	}()
	var errs []error
	errs = append(errs, <-done)
	errs = append(errs, <-done)
	return errors.NewAggregate(errs)
}
