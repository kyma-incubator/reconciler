package serverless

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	serverlessNamespace                    = "kyma-system"
	serverlessSecretName                   = "serverless-registry-config-default"
	serverlessDockerRegistryDeploymentName = "serverless-docker-registry"
	registryHTTPEnvKey                     = "REGISTRY_HTTP_SECRET"
)

type ReconcileCustomAction struct {
	name string
}

func (a *ReconcileCustomAction) Run(svcCtx *service.ActionContext) error {

	logger := svcCtx.Logger
	k8sClient, err := svcCtx.KubeClient.Clientset()
	if err != nil {
		return err
	}
	secret, err := k8sClient.CoreV1().Secrets(serverlessNamespace).Get(svcCtx.Context, serverlessSecretName, metav1.GetOptions{})
	if err == nil && secret != nil {

		logger.Infof("Secret %s found in namespace: %s. Attempting to reusing existing credentials for %s", serverlessSecretName, serverlessNamespace, serverlessDockerRegistryDeploymentName)
		username, err := readSecretKey(secret, "username")
		if err == nil && username != "" {
			svcCtx.Model.Configuration["dockerRegistry.username"] = username
		}
		password, err := readSecretKey(secret, "password")
		if err == nil && password != "" {
			svcCtx.Model.Configuration["dockerRegistry.password"] = password
		}

		deployment, err := k8sClient.AppsV1().Deployments(serverlessNamespace).Get(svcCtx.Context, serverlessDockerRegistryDeploymentName, metav1.GetOptions{})
		if err == nil && deployment != nil {

			logger.Infof("Deployment %s found in namespace: %s. Attempting to reuse existing values", serverlessDockerRegistryDeploymentName, serverlessNamespace)

			if deployment.Spec.Template.ObjectMeta.Annotations == nil {
				deployment.Spec.Template.ObjectMeta.Annotations = map[string]string{}
			}
			rollHash, ok := deployment.Spec.Template.ObjectMeta.Annotations["rollme"]

			if ok && rollHash != "" {
				svcCtx.Model.Configuration["docker-registry.rollme"] = rollHash
			}

			envs := deployment.Spec.Template.Spec.Containers[0].Env
			for _, v := range envs {
				if v.Name == registryHTTPEnvKey && v.Value != "" {
					svcCtx.Model.Configuration["docker-registry.registryHTTPSecret"] = v.Value
				}
			}
		}
	}
	return service.NewInstall(svcCtx.Logger).Invoke(svcCtx.Context, svcCtx.ChartProvider, svcCtx.Model, svcCtx.KubeClient)
}

func readSecretKey(secret *v1.Secret, secretKey string) (string, error) {
	if secret.Data == nil {
		return "", errors.New(fmt.Sprintf("failed to read %s from nil secret data", secretKey))
	}
	secretValue, ok := secret.Data[secretKey]
	if !ok {
		return "", errors.New(fmt.Sprintf("%s is not found in secret", secretKey))
	}
	return string(secretValue), nil
}
