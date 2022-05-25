package cleanup

import (
	"context"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sRetry "k8s.io/client-go/util/retry"
)

func (cmd *CliCleaner) removeCustomResourcesFinalizers() error {
	crds := map[string]schema.GroupVersionResource{}

	if cmd.dropFinalizersOnlyForKymaCRs {
		cmd.logger.Info("Removing finalizers only for custom resources installed by Kyma")

		kymaCRDs, err := cmd.kymaCRDsFinder()
		if err != nil {
			return err
		}

		crManagedByReconciler, err := cmd.findCRDsByLabel(crLabelReconciler)
		if err != nil {
			return err
		}

		crManagedByIstio, err := cmd.findCRDsByLabel(crLabelIstio)
		if err != nil {
			return err
		}

		appendCRDs(crds, kymaCRDs)
		appendCRDs(crds, crManagedByReconciler) //In case current sources doesn't contain Kyma CRD that exist on the cluster (consider upgrades)
		appendCRDs(crds, crManagedByIstio)      //Istio CRD is NOT in Kyma sources
	} else {
		cmd.logger.Info("Removing existing finalizers for all custom resources in the cluster")

		allClusterCRDs, err := cmd.findAllCRDsInCluster()
		if err != nil {
			return err
		}
		appendCRDs(crds, allClusterCRDs)
	}

	var lastErr error
	for key, crdef := range crds {
		err := cmd.removeFinalizersFromAllInstancesOf(crdef) //Continue in case of an error
		if err != nil {
			lastErr = err
			cmd.logger.Infof("Error while dropping finalizers for CustomResourceDefinition \"%s\": %s", key, err.Error())
		}
	}

	return lastErr //return last error (if any)
}

func (cmd *CliCleaner) findAllCRDsInCluster() ([]schema.GroupVersionResource, error) {
	return cmd.findCRDsByLabel("")
}

func (cmd *CliCleaner) findCRDsByLabel(label string) ([]schema.GroupVersionResource, error) {
	res := []schema.GroupVersionResource{}

	crds, err := cmd.apixClient.CustomResourceDefinitions().List(context.Background(), metav1.ListOptions{LabelSelector: label})
	if err != nil && !apierr.IsNotFound(err) {
		return nil, err
	}

	if crds == nil {
		return res, nil
	}

	for _, crd := range crds.Items {
		crdef := schema.GroupVersionResource{
			Group:    crd.Spec.Group,
			Version:  crd.Spec.Version,
			Resource: crd.Spec.Names.Plural,
		}
		res = append(res, crdef)
	}

	return res, nil
}

func (cmd *CliCleaner) removeFinalizersFromAllInstancesOf(crdef schema.GroupVersionResource) error {
	cmd.logger.Infof("Dropping finalizers for all custom resources of type: %s.%s/%s", crdef.Resource, crdef.Group, crdef.Version)
	defer cmd.logger.Infof("Finished dropping finalizers for custom resources of type: %s.%s/%s", crdef.Resource, crdef.Group, crdef.Version)

	customResourceList, err := cmd.k8s.Dynamic().Resource(crdef).Namespace(v1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil && !apierr.IsNotFound(err) {
		return err
	}

	if customResourceList == nil {
		return nil
	}

	for i := range customResourceList.Items {
		instance := customResourceList.Items[i]
		retryErr := k8sRetry.RetryOnConflict(k8sRetry.DefaultRetry, func() error { return cmd.removeCustomResourceFinalizers(crdef, instance) })
		if retryErr != nil {
			return errors.Wrapf(retryErr, "deleting finalizer for %s.%s/%s \"%s\" failed", crdef.Resource, crdef.Group, crdef.Version, instance.GetName())
		}
	}

	return nil
}

func (cmd *CliCleaner) removeCustomResourceFinalizers(crdef schema.GroupVersionResource, instance unstructured.Unstructured) error {
	// Retrieve the latest version of Custom Resource before attempting update
	// RetryOnConflict uses exponential backoff to avoid exhausting the apiserver
	res, err := cmd.k8s.Dynamic().Resource(crdef).Namespace(instance.GetNamespace()).Get(context.Background(), instance.GetName(), metav1.GetOptions{})
	if err != nil && !apierr.IsNotFound(err) {
		return err
	}
	if res == nil {
		return nil
	}

	if len(res.GetFinalizers()) > 0 {
		cmd.logger.Infof("Found finalizers for \"%s\" %s, deleting", res.GetName(), instance.GetKind())

		res.SetFinalizers(nil)
		_, err := cmd.k8s.Dynamic().Resource(crdef).Namespace(res.GetNamespace()).Update(context.Background(), res, metav1.UpdateOptions{})
		if err != nil {
			return err
		}

		cmd.logger.Infof("Deleted finalizer for \"%s\" %s", res.GetName(), instance.GetKind())
	}

	if !cmd.keepCRDs {
		err = cmd.k8s.Dynamic().Resource(crdef).Namespace(res.GetNamespace()).Delete(context.Background(), res.GetName(), metav1.DeleteOptions{})
		if err != nil && !apierr.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func appendCRDs(m map[string]schema.GroupVersionResource, list []schema.GroupVersionResource) {
	for _, gvr := range list {
		key := gvr.Resource + "." + gvr.Group + "/" + gvr.Version
		m[key] = gvr
	}
}
