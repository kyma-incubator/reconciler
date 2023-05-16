package serverless

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"k8s.io/apimachinery/pkg/api/errors"
)

var istioResources = []kubernetes.Resource{
	{Kind: "VirtualService", Name: "serverless-docker-registry", Namespace: serverlessNamespace},
	{Kind: "DestinationRule", Name: "serverless-docker-registry", Namespace: serverlessNamespace},
	{Kind: "ConfigMap", Name: "serverless-docker-registry-cert-patch", Namespace: serverlessNamespace},
	{Kind: "DaemonSet", Name: "serverless-docker-registry-self-signed-cert", Namespace: serverlessNamespace},
	{Kind: "ServiceAccount", Name: "serverless-self-signed-cert", Namespace: serverlessNamespace},
	{Kind: "ClusterRole", Name: "serverless-self-signed-cert", Namespace: serverlessNamespace},
	{Kind: "ClusterRoleBinding", Name: "serverless-self-signed-cert", Namespace: serverlessNamespace},
}

type ResourceCleanupAction struct {
	name      string
	resources []kubernetes.Resource
}

func (a *ResourceCleanupAction) Run(svcCtx *service.ActionContext) error {
	logger := svcCtx.Logger.With("action", a.name)
	kubeClient := svcCtx.KubeClient

	for _, res := range a.resources {
		_, err := kubeClient.DeleteResource(
			svcCtx.Context,
			res.Kind,
			res.Name,
			res.Namespace,
		)

		if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("Error while removing resource %s: %s", res.String(), err.Error())
		}

		logger.Infof("Resource %s removed", res.String())
	}

	return nil
}
