package busola_migrator

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
)

type client struct {
}

func NewVirtSvcClient() *client {
	return &client{}
}

func (c *client) GetVirtSvcHosts(ctx context.Context, restClient rest.Interface, name, namespace string) ([]string, error) {
	r, err := restClient.
		Get().
		AbsPath("/apis/networking.istio.io/v1alpha3").
		Resource("virtualservices").
		Namespace(namespace).
		Name(name).
		DoRaw(ctx)

	if err != nil {
		return nil, err
	}

	var virtSvc map[string]interface{}
	if err := json.Unmarshal(r, &virtSvc); err != nil {
		return nil, err
	}
	fmt.Println(string(r))
	fmt.Println(virtSvc["spec"])
	return extractHosts(virtSvc)
}

func (c *client) PatchVirtSvc(ctx context.Context, restClient rest.Interface, name, namespace string, patch virtualServicePatch) error {
	out, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	_, err = restClient.
		Patch(types.MergePatchType).
		AbsPath("/apis/networking.istio.io/v1alpha3").
		Resource("virtualservices").
		Namespace(namespace).
		Name(name).
		Body(out).
		DoRaw(ctx)
	return errors.Wrapf(err, "while patching virtual service: %s, in namespace: %s", name, namespace)
}

func extractHosts(vs map[string]interface{}) ([]string, error) {
	tmpSpec := vs["spec"]
	spec, ok := tmpSpec.(map[string]interface{})
	if !ok {
		return nil, nil
	}

	genericHosts, ok := spec["hosts"].([]interface{})
	if !ok {
		return nil, nil
	}

	var hosts []string
	for _, genericHost := range genericHosts {
		if host, ok := genericHost.(string); ok {
			hosts = append(hosts, host)
		} else {
			//TODO: return error with significant message
			return nil, errors.New(fmt.Sprintf("%+v is not a string", genericHost))
		}
	}

	return hosts, nil
}
