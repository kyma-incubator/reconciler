package busolamigrator

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/rest"
)

type virtualServicePatch struct {
	Spec specPatch `json:"spec"`
}

type specPatch struct {
	Hosts []string `json:"hosts"`
}

type VirtualSvcMeta struct {
	Name      string
	Namespace string
}

type VirtSvcPreReconcilePatch struct {
	name            string
	virtSvcsToPatch []VirtualSvcMeta
	suffix          string
	virtSvcClient   VirtSvcClient
}

var _ service.Action = &VirtSvcPreReconcilePatch{}

func NewVirtualServicePreInstallPatch(virtualSvcs []VirtualSvcMeta, suffix string) *VirtSvcPreReconcilePatch {
	client := NewVirtSvcClient()
	return &VirtSvcPreReconcilePatch{"pre-reconciler", virtualSvcs, suffix, client}
}

func (p *VirtSvcPreReconcilePatch) Run(helper *service.ActionContext) error {
	ctx := context.TODO()
	logger := helper.Logger
	clientSet, err := helper.KubeClient.Clientset()
	if err != nil {
		return errors.Wrapf(err, "while getting client set from kubeclient")
	}
	restClient := clientSet.Discovery().RESTClient()

	logger.Infof("Launching pre install busola migrator job, version: %s ", helper.Task.Version)
	for _, virtSvcToPatch := range p.virtSvcsToPatch {
		logger.Infof("Patching virtual service: %s in namespace: %s", virtSvcToPatch.Name, virtSvcToPatch.Namespace)
		if err := p.patchVirtSvc(ctx, restClient, virtSvcToPatch.Name, virtSvcToPatch.Namespace, logger); err != nil {
			return errors.Wrapf(err, "while patching virtual service: %s, in namespace: %s", virtSvcToPatch.Name, virtSvcToPatch.Namespace)
		}
		logger.Infof("Finished patching of virtual service : %s in namespace: %s", virtSvcToPatch.Name, virtSvcToPatch.Namespace)
	}
	logger.Info("Finished pre reconciler busola migrator job")
	return nil
}

func (p *VirtSvcPreReconcilePatch) patchVirtSvc(ctx context.Context, kubeRestClient rest.Interface, name, namespace string, logger *zap.SugaredLogger) error {
	hosts, err := p.virtSvcClient.GetVirtSvcHosts(ctx, kubeRestClient, name, namespace)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			logger.Infof("Given virtual service: %s in namespace: %s not found, which is okay", name, namespace)
			return nil
		}
		return errors.Wrapf(err, "while getting virtual service: %s, in namespace: %s", name, namespace)
	}

	if len(hosts) < 1 {
		return errors.New(fmt.Sprintf("hosts is empty in virtual service: %s, in namespace: %s", name, namespace))
	}

	has, err := hasSuffix(hosts[0], p.suffix)
	if err != nil {
		return errors.Wrapf(err, "while checking suffix in host: %s, in namespace: %s", name, namespace)
	}

	if has {
		logger.Infof("Virtual service already patched: %s, in namespace: %s", name, namespace)
		return nil
	}

	host, err := addSuffix(hosts[0], p.suffix)
	if err != nil {
		return errors.Wrapf(err, "while appending suffix to host in virtual service: %s, in namespace: %s", name, namespace)
	}
	hosts[0] = host
	patch := virtualServicePatch{Spec: specPatch{Hosts: hosts}}

	err = p.virtSvcClient.PatchVirtSvc(ctx, kubeRestClient, name, namespace, patch)
	return errors.Wrap(err, "while patching virtual service")
}

func hasSuffix(host, suffix string) (bool, error) {
	splittedHost := strings.Split(host, ".")
	if len(splittedHost) < 1 {
		return false, errors.Errorf("something is wrong with host name: %s", host)
	}
	return strings.HasSuffix(splittedHost[0], suffix), nil
}

func addSuffix(host, suffix string) (string, error) {
	if _, err := url.Parse(host); err != nil {
		return "", err
	}
	splittedHost := strings.Split(host, ".")
	if len(splittedHost) < 1 {
		return "", errors.Errorf("something is wrong with host name: %s", host)
	}
	splittedHost[0] = fmt.Sprintf("%s%s", splittedHost[0], suffix)
	return strings.Join(splittedHost, "."), nil
}
