package scmigration

import (
	"context"

	"github.com/SAP/sap-btp-service-operator/client/sm"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/scheme"
)

const svCatGroupName = "servicecatalog.k8s.io"
const svCatGroupVersion = "v1beta1"
const operatorGroupName = "services.cloud.sap.com"
const operatorGroupVersion = "v1alpha1"

func getSMClient(ctx context.Context, secret *v1.Secret) (sm.Client, error) {
	secretData := secret.Data
	return sm.NewClient(ctx, &sm.ClientConfig{
		ClientID:       string(secretData["clientid"]),
		ClientSecret:   string(secretData["clientsecret"]),
		URL:            string(secretData["url"]),
		TokenURL:       string(secretData["tokenurl"]),
		TokenURLSuffix: "/oauth/token",
		SSLDisabled:    false,
	}, nil)
}

func getK8sClient(config *rest.Config, groupName, groupVersion string) *rest.RESTClient {
	opcrdConfig := *config
	opcrdConfig.ContentConfig.GroupVersion = &schema.GroupVersion{Group: groupName, Version: groupVersion}
	opcrdConfig.APIPath = "/apis"
	opcrdConfig.NegotiatedSerializer = serializer.NewCodecFactory(scheme.Scheme)
	opcrdConfig.UserAgent = rest.DefaultKubernetesUserAgent()

	operatorClient, err := rest.UnversionedRESTClientFor(&opcrdConfig)
	cobra.CheckErr(err)
	return operatorClient
}
