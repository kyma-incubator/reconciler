package busola_migrator

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"strings"
)

type virtSvcClient interface {
	GetVirtSvcHosts(ctx context.Context, restClient rest.Interface, name, namespace string) ([]string, error)
	PatchVirtSvc(ctx context.Context, restClient rest.Interface, name, namespace string, patch virtualServicePatch) error
}

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

type VirtualServicePreInstallPatch struct {
	virtSvcsToPatch []VirtualSvcMeta
	suffix          string
	virtSvcClient   virtSvcClient
}

func NewVirtualServicePreInstallPatch(virtualSvcs []VirtualSvcMeta, suffix string) *VirtualServicePreInstallPatch {
	client := NewVirtSvcClient()
	return &VirtualServicePreInstallPatch{virtualSvcs, suffix, client}
}

func (p *VirtualServicePreInstallPatch) Run(version string, kubeClient kubernetes.Interface) error {
	ctx := context.TODO()

	for _, virtSvcToPatch := range p.virtSvcsToPatch {
		if err := p.patchVirtSvc(ctx, kubeClient.Discovery().RESTClient(), virtSvcToPatch.Name, virtSvcToPatch.Namespace); err != nil {
			return errors.Wrapf(err, "while patching virtual service: %s, in namespace: %s", virtSvcToPatch.Name, virtSvcToPatch.Namespace)
		}
	}
	return nil
}

func (p *VirtualServicePreInstallPatch) patchVirtSvc(ctx context.Context, kubeRestClient rest.Interface, name, namespace string) error {

	hosts, err := p.virtSvcClient.GetVirtSvcHosts(ctx, kubeRestClient, name, namespace)
	if err != nil {
		return errors.Wrapf(err, "while getting virtual service: %s, in namespace: %s", name, namespace)
	}
	if len(hosts) < 1 {
		return errors.New(fmt.Sprintf("hosts is empty in virtual service: %s, in namespace: %s", name, namespace))
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

func addSuffix(host, suffix string) (string, error) {
	splittedHost := strings.Split(host, ".")
	if len(splittedHost) < 1 {
		return "", errors.Errorf("host name is incorrect: %s", host)
	}
	splittedHost[0] = fmt.Sprintf("%s%s", splittedHost[0], suffix)
	return strings.Join(splittedHost, "."), nil
}
