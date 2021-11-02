package serverless

import (
	"fmt"
	"strconv"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
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
		return errors.Wrap(err, "while getting clientset")
	}
	secret, err := k8sClient.CoreV1().Secrets(serverlessNamespace).Get(svcCtx.Context, serverlessSecretName, metav1.GetOptions{})
	if err != nil {
		logger.Errorf("Error while fetching existing docker registry secret [%s]... Secret will be re-generated", err.Error())
	} else if secret != nil {
		logger.Infof("Secret %s found in namespace: %s. Attempting to reusing existing credentials for %s", serverlessSecretName, serverlessNamespace, serverlessDockerRegistryDeploymentName)
		setOverrideFromSecret(logger, secret, svcCtx.Task.Configuration, "username", "dockerRegistry.username")
		setOverrideFromSecret(logger, secret, svcCtx.Task.Configuration, "password", "dockerRegistry.password")
		setOverrideFromSecret(logger, secret, svcCtx.Task.Configuration, "isInternal", "dockerRegistry.enableInternal")
		setOverrideFromSecret(logger, secret, svcCtx.Task.Configuration, "registryAddress", "dockerRegistry.registryAddress")
		setOverrideFromSecret(logger, secret, svcCtx.Task.Configuration, "serverAddress", "dockerRegistry.serverAddress")

		deployment, err := k8sClient.AppsV1().Deployments(serverlessNamespace).Get(svcCtx.Context, serverlessDockerRegistryDeploymentName, metav1.GetOptions{})
		if err != nil {
			logger.Errorf("Error while fetching existing docker registry deployment [%s]... Deployment will be re-generated", err.Error())
		} else if deployment != nil {
			setOverridesFromDeployment(deployment, svcCtx.Task.Configuration)
		}
	}
	return service.NewInstall(svcCtx.Logger).Invoke(svcCtx.Context, svcCtx.ChartProvider, svcCtx.Task, svcCtx.KubeClient)
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

func setOverrideFromSecret(logger *zap.SugaredLogger, secret *v1.Secret, configuration map[string]interface{}, secretKey string, overridePath string) {
	secretValue, err := readSecretKey(secret, secretKey)
	if err != nil {
		logger.Errorf("Error while fetching %s from secret... Override for path [%s] will be generated : [%s]", secretKey, overridePath, err.Error())
		return
	}
	if secretValue != "" {
		if secretValue == "true" || secretValue == "false" {
			configuration[overridePath], _ = strconv.ParseBool(secretValue)
		} else {
			configuration[overridePath] = secretValue
		}
	}
}

func setOverridesFromDeployment(deployment *appsv1.Deployment, configuration map[string]interface{}) {
	if deployment.Spec.Template.ObjectMeta.Annotations == nil {
		deployment.Spec.Template.ObjectMeta.Annotations = map[string]string{}
	}
	rollHash, ok := deployment.Spec.Template.ObjectMeta.Annotations["rollme"]
	if ok && rollHash != "" {
		configuration["docker-registry.rollme"] = rollHash
	}
	envs := deployment.Spec.Template.Spec.Containers[0].Env
	for _, v := range envs {
		if v.Name == registryHTTPEnvKey && v.Value != "" {
			configuration["docker-registry.registryHTTPSecret"] = v.Value
		}
	}
}
