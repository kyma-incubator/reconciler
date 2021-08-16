package busolamigrator

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
)

type VirtSvcClient interface {
	GetVirtSvcHosts(ctx context.Context, restClient rest.Interface, name, namespace string) ([]string, error)
	PatchVirtSvc(ctx context.Context, restClient rest.Interface, name, namespace string, patch virtualServicePatch) error
}

var _ VirtSvcClient = &client{}

type client struct {
}

func NewVirtSvcClient() VirtSvcClient {
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
	resp, err := restClient.
		Patch(types.MergePatchType).
		AbsPath(istioCRPath).
		Resource(resourceName).
		Namespace(namespace).
		Name(name).
		Body(out).
		DoRaw(ctx)
	if err != nil {
		return errors.Wrapf(err, "while patching virtual service: %s, in namespace: %s, response: %s", name, namespace, string(resp))
	}
	return nil
}
