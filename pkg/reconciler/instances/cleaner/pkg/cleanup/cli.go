package cleanup

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/avast/retry-go"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	apixv1beta1client "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	k8sRetry "k8s.io/client-go/util/retry"
)

const (
	defaultHTTPTimeout = 30 * time.Second //Expose as a configuration option if necessary
	namespaceTimeout   = 6 * time.Minute  //Expose as a configuration option if necessary
	crLabelReconciler  = "reconciler.kyma-project.io/managed-by=reconciler"
	crLabelIstio       = "install.operator.istio.io/owning-resource-namespace=istio-system"
	kymaNamespace      = "kyma-system"
)

//Implements cleanup logic taken from kyma-cli
type CliCleaner struct {
	k8s              KymaKube
	apixClient       apixv1beta1client.ApiextensionsV1beta1Interface
	keepCRDs         bool
	namespaces       []string
	namespaceTimeout time.Duration
	logger           *zap.SugaredLogger
}

func NewCliCleaner(kubeconfigData string, namespaces []string, logger *zap.SugaredLogger) (*CliCleaner, error) {

	kymaKube, err := NewFromConfigWithTimeout(kubeconfigData, defaultHTTPTimeout)
	if err != nil {
		return nil, err
	}

	var apixClient *apixv1beta1client.ApiextensionsV1beta1Client
	if apixClient, err = apixv1beta1client.NewForConfig(kymaKube.RestConfig()); err != nil {
		return nil, err
	}

	return &CliCleaner{kymaKube, apixClient, true, namespaces, namespaceTimeout, logger}, nil
}

//Run runs the command
func (cmd *CliCleaner) Run() error {

	if err := cmd.deletePVCSAndWait(kymaNamespace); err != nil {
		return err
	}

	if err := cmd.deleteKymaNamespaces(); err != nil {
		return err
	}

	if err := cmd.waitForNamespaces(); err != nil {
		return err
	}

	return nil
}

func (cmd *CliCleaner) removeServerlessCredentialFinalizers() error {
	secrets, err := cmd.k8s.Static().CoreV1().Secrets(v1.NamespaceAll).List(context.Background(), metav1.ListOptions{LabelSelector: "serverless.kyma-project.io/config=credentials"})
	if err != nil && !apierr.IsNotFound(err) {
		return err
	}

	if secrets == nil {
		return nil
	}

	for i := range secrets.Items {
		secret := secrets.Items[i]

		if len(secret.GetFinalizers()) <= 0 {
			continue
		}

		secret.SetFinalizers(nil)
		if _, err := cmd.k8s.Static().CoreV1().Secrets(secret.GetNamespace()).Update(context.Background(), &secret, metav1.UpdateOptions{}); err != nil {
			return err
		}

		cmd.logger.Info(fmt.Sprintf("Deleted finalizer from \"%s\" Secret", secret.GetName()))
	}

	return nil
}

func (cmd *CliCleaner) removeCustomResourcesFinalizers() error {
	if err := cmd.removeCustomResourceFinalizersByLabel(crLabelReconciler); err != nil {
		return err
	}
	if err := cmd.removeCustomResourceFinalizersByLabel(crLabelIstio); err != nil {
		return err
	}

	return nil
}

func (cmd *CliCleaner) removeCustomResourceFinalizersByLabel(label string) error {
	crds, err := cmd.apixClient.CustomResourceDefinitions().List(context.Background(), metav1.ListOptions{LabelSelector: label})
	if err != nil && !apierr.IsNotFound(err) {
		return err
	}
	if crds == nil {
		return nil
	}

	for _, crd := range crds.Items {
		gvr := schema.GroupVersionResource{
			Group:    crd.Spec.Group,
			Version:  crd.Spec.Version,
			Resource: crd.Spec.Names.Plural,
		}

		customResourceList, err := cmd.k8s.Dynamic().Resource(gvr).Namespace(v1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
		if err != nil && !apierr.IsNotFound(err) {
			return err
		}
		if customResourceList == nil {
			continue
		}

		for i := range customResourceList.Items {
			cr := customResourceList.Items[i]
			retryErr := k8sRetry.RetryOnConflict(k8sRetry.DefaultRetry, func() error { return cmd.removeCustomResourceFinalizers(gvr, cr) })
			if retryErr != nil {
				return errors.Wrap(retryErr, "deleting finalizer failed:")
			}
		}
	}

	return nil
}

func (cmd *CliCleaner) removeCustomResourceFinalizers(gvr schema.GroupVersionResource, cr unstructured.Unstructured) error {
	// Retrieve the latest version of Custom Resource before attempting update
	// RetryOnConflict uses exponential backoff to avoid exhausting the apiserver
	res, err := cmd.k8s.Dynamic().Resource(gvr).Namespace(cr.GetNamespace()).Get(context.Background(), cr.GetName(), metav1.GetOptions{})
	if err != nil && !apierr.IsNotFound(err) {
		return err
	}
	if res == nil {
		return nil
	}

	if len(res.GetFinalizers()) > 0 {
		cmd.logger.Info(fmt.Sprintf("Deleting finalizer for \"%s\" %s", res.GetName(), cr.GetKind()))

		res.SetFinalizers(nil)
		_, err := cmd.k8s.Dynamic().Resource(gvr).Namespace(res.GetNamespace()).Update(context.Background(), res, metav1.UpdateOptions{})
		if err != nil {
			return err
		}

		cmd.logger.Info(fmt.Sprintf("Deleted finalizer for \"%s\" %s", res.GetName(), res.GetKind()))
	}

	if !cmd.keepCRDs {
		err = cmd.k8s.Dynamic().Resource(gvr).Namespace(res.GetNamespace()).Delete(context.Background(), res.GetName(), metav1.DeleteOptions{})
		if err != nil && !apierr.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (cmd *CliCleaner) deleteKymaNamespaces() error {
	cmd.logger.Info("Deleting Kyma Namespaces")

	var wg sync.WaitGroup
	wg.Add(len(cmd.namespaces))
	finishedCh := make(chan bool)
	errorCh := make(chan error)

	for _, namespace := range cmd.namespaces {
		go func(ns string) {
			defer wg.Done()
			err := retry.Do(func() error {
				cmd.logger.Info(fmt.Sprintf("Deleting Namespace \"%s\"", ns))
				//HACK: drop kyma-system finalizers -> TBD: remove this hack after issue is fixed (https://github.com/kyma-project/kyma/issues/10470)
				if ns == kymaNamespace {

					_, err := cmd.k8s.Static().CoreV1().Namespaces().Finalize(context.Background(), &v1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name:       ns,
							Finalizers: []string{},
						},
					}, metav1.UpdateOptions{})
					if err != nil {
						errorCh <- err
					}
				}

				if err := cmd.k8s.Static().CoreV1().Namespaces().Delete(context.Background(), ns, metav1.DeleteOptions{}); err != nil && !apierr.IsNotFound(err) {
					errorCh <- err
				}

				nsT, err := cmd.k8s.Static().CoreV1().Namespaces().Get(context.Background(), ns, metav1.GetOptions{})
				if err != nil && !apierr.IsNotFound(err) {
					errorCh <- err
				} else if apierr.IsNotFound(err) {
					return nil
				}

				return errors.Wrapf(err, "\"%s\" Namespace still exists in \"%s\" Phase", nsT.Name, nsT.Status.Phase)
			})
			if err != nil {
				errorCh <- err
				return
			}

			cmd.logger.Info(fmt.Sprintf("\"%s\" Namespace is removed", ns))
		}(namespace)
	}

	go func() {
		wg.Wait()
		close(errorCh)
		close(finishedCh)
	}()

	// process deletion results
	var errWrapped error
	for {
		select {
		case <-finishedCh:
			if errWrapped == nil {
				cmd.logger.Info("All Kyma Namespaces marked for deletion")
			}
			return errWrapped
		case err := <-errorCh:
			if err != nil {
				if errWrapped == nil {
					errWrapped = err
				} else {
					errWrapped = errors.Wrap(err, errWrapped.Error())
				}
			}
		}
	}
}

func (cmd *CliCleaner) waitForNamespaces() error {

	cmd.logger.Info("Waiting for Namespace deletion")

	timeout := time.After(cmd.namespaceTimeout)
	poll := time.NewTicker(6 * time.Second)
	defer poll.Stop()
	for {
		select {
		case <-timeout:
			return errors.New("Timed out while waiting for Namespace deletion")
		case <-poll.C:
			if err := cmd.removeFinalizers(); err != nil {
				return err
			}
			ok, err := cmd.checkKymaNamespaces()
			if err != nil {
				return err
			} else if ok {
				return nil
			}
		}
	}
}

func (cmd *CliCleaner) removeFinalizers() error {

	if err := cmd.removeServerlessCredentialFinalizers(); err != nil {
		return err
	}

	if err := cmd.removeCustomResourcesFinalizers(); err != nil {
		return err
	}

	return nil
}

func (cmd *CliCleaner) checkKymaNamespaces() (bool, error) {
	namespaceList, err := cmd.k8s.Static().CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return false, err
	}

	if namespaceList.Size() == 0 {
		cmd.logger.Info("No remaining Kyma Namespaces found")
		return true, nil
	}

	for i := range namespaceList.Items {
		if contains(cmd.namespaces, namespaceList.Items[i].Name) {
			cmd.logger.Info(fmt.Sprintf("Namespace %s still in state '%s'", namespaceList.Items[i].Name, namespaceList.Items[i].Status.Phase))
			return false, nil
		}
	}

	cmd.logger.Info("No remaining Kyma Namespaces found")

	return true, nil
}

func contains(items []string, item string) bool {
	for _, i := range items {
		if i == item {
			return true
		}
	}
	return false
}

//taken from github.com/kyma-project/cli/internal/kube/kube.go
type KymaKube interface {
	Static() kubernetes.Interface
	Dynamic() dynamic.Interface

	// RestConfig provides the REST configuration of the kubernetes client
	RestConfig() *rest.Config
}

// NewFromConfigWithTimeout creates a new Kubernetes client based on the given Kubeconfig provided by a file (out-of-cluster config).
// Allows to set a custom timeout for the Kubernetes HTTP client.
func NewFromConfigWithTimeout(kubeconfigData string, t time.Duration) (KymaKube, error) {
	config, err := restConfig(kubeconfigData)
	if err != nil {
		return nil, err
	}

	config.Timeout = t

	sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	dClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &client{
			static:  sClient,
			dynamic: dClient,
			restCfg: config,
		},
		nil
}

//taken from github.com/kyma-project/cli/internal/kube/client.go
//client is the default KymaKube implementation
type client struct {
	static  kubernetes.Interface
	dynamic dynamic.Interface
	restCfg *rest.Config
}

func (c *client) Static() kubernetes.Interface {
	return c.static
}

func (c *client) Dynamic() dynamic.Interface {
	return c.dynamic
}

func (c *client) RestConfig() *rest.Config {
	return c.restCfg
}

// restConfig loads the rest configuration needed by k8s clients to interact with clusters based on the kubeconfig.
// Loading rules are based on standard defined kubernetes config loading.
func restConfig(kubeconfigData string) (*rest.Config, error) {
	cfg, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfigData))
	if err != nil {
		return nil, err
	}
	cfg.WarningHandler = rest.NoWarnings{}
	return cfg, nil
}
