package busola_migrator

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"strings"

	//istio_client "istio.io/client-go"
)

type virtualServicePatch struct {
	Spec specPatch `json:"spec"`
}

type specPatch struct {
	Hosts []string `json:"hosts"`
}

type VirtualServicePreInstallPatch struct {
	dexVirtSvcName     string
	dexNamespace       string
	consoleVirtSvcName string
	consoleNamespace   string
	suffix             string
}

func NewVirtualServicePreInstallPatch(dexVirtSvcName, dexNamespace, consoleVirtSvcName, consoleNamespace, suffix string) *VirtualServicePreInstallPatch {
	return &VirtualServicePreInstallPatch{dexVirtSvcName, dexNamespace, consoleVirtSvcName, consoleNamespace, suffix,
	}
}

func (p *VirtualServicePreInstallPatch) Run(version string, kubeClient *kubernetes.Clientset) error {
	if err := p.patchVirtualServiceHost(kubeClient, p.dexVirtSvcName, p.dexNamespace); err != nil {
		return err
	}
	if err := p.patchVirtualServiceHost(kubeClient, p.consoleVirtSvcName, p.consoleNamespace); err != nil {
		return err
	}
	return nil
}

func (p *VirtualServicePreInstallPatch) patchVirtualServiceHost(kubeClient *kubernetes.Clientset, virtSvcName, namespace string) error {
	ctx := context.TODO()
	r, err := kubeClient.RESTClient().
		Get().
		//networking.istio.io
		AbsPath("/apis/networking.istio.io/v1alpha3").
		Resource("virtualservices").
		Namespace(namespace).
		Name(virtSvcName).
		DoRaw(ctx)

	if err != nil {
		return err
	}

	var virtSvc map[string]interface{}
	if err := json.Unmarshal(r, &virtSvc); err != nil {
		return err
	}
	fmt.Println(string(r))
	fmt.Println(virtSvc["spec"])
	hosts, err := extractHosts(virtSvc)
	if err != nil {
		return err
		return errors.New("Could find host in virtual service")
	}

	hosts[0] = addSuffix(hosts[0], p.suffix)
	patch := virtualServicePatch{
		Spec: specPatch{Hosts: hosts},
	}

	err = p.patchVirtualService(kubeClient, virtSvcName, namespace, patch)
	return err
}

func (p* VirtualServicePreInstallPatch) patchVirtualService(kubeClient *kubernetes.Clientset, virtSvcName, namespace string,vsPatch virtualServicePatch )  error{
	ctx := context.TODO()

	out, err := json.Marshal(vsPatch)
	if err != nil {
		return err
	}
	_, err = kubeClient.RESTClient().
		Patch(types.MergePatchType).
		AbsPath("/apis/networking.istio.io/v1alpha3").
		Resource("virtualservices").
		Namespace(namespace).
		Name(virtSvcName).
		Body(out).
		DoRaw(ctx)
	if err != nil {
		return err
	}
	return nil
}

func extractHosts(vs map[string]interface{}) ([]string, error) {
	tmpSpec := vs["spec"]
	spec, ok := tmpSpec.(map[string]interface{})
	if !ok {
		return nil, nil
	}

	genericHosts, ok := spec["hosts"].([]interface{})
	if !ok {
		return nil,nil
	}

	var hosts []string
	for _, genericHost := range genericHosts {
		if host, ok := genericHost.(string); ok {
			hosts = append(hosts, host)
		} else {
			//TODO: return error with significant message
			return nil, errors.New(fmt.Sprintf("%+v is not a string",genericHost))
		}
	}


	return hosts, nil
}

func addSuffix(host, suffix string) string {
	splittedHost := strings.Split(host, ".")

	splittedHost[0] = fmt.Sprintf("%s%s", splittedHost[0], suffix)
	return strings.Join(splittedHost, ".")
}
