package busola_migrator

import (
	"context"
	"encoding/json"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
)

type client struct {
}

func NewVirtSvcClient() *client {
	return &client{}
}

const (
	istioCRPath  = "/apis/networking.istio.io/v1alpha3"
	resourceName = "virtualservices"
)

type virtSvc struct {
	Spec virtSvcSpec `json:"spec"`
}
type virtSvcSpec struct {
	Hosts []string `json:"hosts"`
}

func (c *client) GetVirtSvcHosts(ctx context.Context, restClient rest.Interface, name, namespace string) ([]string, error) {
	r, err := restClient.
		Get().
		AbsPath(istioCRPath).
		Resource(resourceName).
		Namespace(namespace).
		Name(name).
		DoRaw(ctx)

	if err != nil {
		return nil, err
	}

	var virtSvc virtSvc
	if err := json.Unmarshal(r, &virtSvc); err != nil {
		return nil, errors.Wrap(err, "while unmarshalling virtual service")
	}
	return virtSvc.Spec.Hosts, nil
}

func (c *client) PatchVirtSvc(ctx context.Context, restClient rest.Interface, name, namespace string, patch virtualServicePatch) error {
	out, err := json.Marshal(patch)
	if err != nil {
		return errors.Wrap(err, "while marshalling virtual service patch")
	}
	_, err = restClient.
		Patch(types.MergePatchType).
		AbsPath(istioCRPath).
		Resource(resourceName).
		Namespace(namespace).
		Name(name).
		Body(out).
		DoRaw(ctx)

	return errors.Wrapf(err, "while patching virtual service: %s, in namespace: %s", name, namespace)
}

//func extractHosts(vs virtSvc) ([]string, error) {
//	//tmpSpec := vs.Spec
//	//spec, ok := tmpSpec.(map[string]interface{})
//	//if !ok {
//	//	return nil, errors.New("could find `{.spec}` field in virtual service")
//	//}
//
//	//genericHosts, ok := spec["hosts"].([]interface{})
//	//if !ok {
//	//	return nil, errors.New("could find `{.spec.hosts}` field in virtual service")
//	//}
//
//	hosts  = vs.Spec.Hosts
//	for _, genericHost := range genericHosts {
//		if host, ok := genericHost.(string); ok {
//			hosts = append(hosts, host)
//		} else {
//			return nil, errors.New(fmt.Sprintf("%+v is not a string", genericHost))
//		}
//	}
//
//	return hosts, nil
//}
