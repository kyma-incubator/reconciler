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
	apixV1ClientSet "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultHTTPTimeout = 30 * time.Second //Expose as a configuration option if necessary
	namespaceTimeout   = 6 * time.Minute  //Expose as a configuration option if necessary
	crLabelReconciler  = "reconciler.kyma-project.io/managed-by=reconciler"
	crLabelIstio       = "install.operator.istio.io/owning-resource-namespace=istio-system"
	kymaNamespace      = "kyma-system"
)

//KymaCRDsFinder returns a list of all CRDs defined explicitly in Kyma sources/charts.
type KymaCRDsFinder func() ([]schema.GroupVersionResource, error)

//Implements cleanup logic
type CliCleaner struct {
	k8s                          KymaKube
	apixClient                   apixV1ClientSet.ApiextensionsV1Interface
	keepCRDs                     bool
	dropFinalizersOnlyForKymaCRs bool
	kymaCRDsFinder               KymaCRDsFinder
	namespaces                   []string
	namespaceTimeout             time.Duration
	logger                       *zap.SugaredLogger
}

func NewCliCleaner(kubeconfigData string, namespaces []string, logger *zap.SugaredLogger, dropFinalizersOnlyForKymaCRs bool, crdsFinder KymaCRDsFinder) (*CliCleaner, error) {

	kymaKube, err := NewFromConfigWithTimeout(kubeconfigData, defaultHTTPTimeout)
	if err != nil {
		return nil, err
	}

	var apixClient *apixV1ClientSet.ApiextensionsV1Client
	if apixClient, err = apixV1ClientSet.NewForConfig(kymaKube.RestConfig()); err != nil {
		return nil, err
	}

	return &CliCleaner{kymaKube, apixClient, true, dropFinalizersOnlyForKymaCRs, crdsFinder, namespaces, namespaceTimeout, logger}, nil
}

//Run runs the command
func (cmd *CliCleaner) Run() error {

	if err := cmd.deletePVCSAndWait(kymaNamespace); err != nil {
		return err
	}

	if err := cmd.removeResourcesFinalizers(); err != nil {
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
			if err := cmd.removeResourcesFinalizers(); err != nil {
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

func (cmd *CliCleaner) removeResourcesFinalizers() error {

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
