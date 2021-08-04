package busola_migrator

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"strings"

	//istio_client "istio.io/client-go"
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

	for _, virtSvcToPatch := range p.virtSvcsToPatch {
		if err := p.PatchVirtSvc(kubeClient.Discovery().RESTClient(), virtSvcToPatch.Name, virtSvcToPatch.Namespace); err != nil {
			return err
		}
	}
	return nil
}

func (p *VirtualServicePreInstallPatch) PatchVirtSvc(kubeRestClient rest.Interface, virtSvcName, namespace string) error {
	ctx := context.TODO()

	hosts, err := p.virtSvcClient.GetVirtSvcHosts(ctx, kubeRestClient, virtSvcName, namespace)
	if err != nil {
		return err
	}

	hosts[0] = addSuffix(hosts[0], p.suffix)
	patch := virtualServicePatch{
		Spec: specPatch{Hosts: hosts},
	}

	err = p.virtSvcClient.PatchVirtSvc(ctx, kubeRestClient, virtSvcName, namespace, patch)
	return errors.Wrap(err,"while patching virtual service")
}

func addSuffix(host, suffix string) string {
	splittedHost := strings.Split(host, ".")

	splittedHost[0] = fmt.Sprintf("%s%s", splittedHost[0], suffix)
	return strings.Join(splittedHost, ".")
}
