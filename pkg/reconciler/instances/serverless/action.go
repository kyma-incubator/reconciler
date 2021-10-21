package serverless

import (
	"crypto/rand"
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	serverlessNamespace                    = "kyma-system"
	serverlessSecretName                   = "serverless-registry-config-default"
	serverlessDockerRegistryDeploymentName = "serverless-docker-registry"
	usernameSize                           = 20
	passwordSize                           = 40
	rollMeSize                             = 5
)

type ServerlessReconcileCustomAction struct {
	name string
}

func (a *ServerlessReconcileCustomAction) Run(svcCtx *service.ActionContext) error {

	var err error
	var username string
	var password string
	var rollHash string

	logger := svcCtx.Logger
	k8sClient, err := svcCtx.KubeClient.Clientset()
	if err != nil {
		return err
	}

	secret, err := k8sClient.CoreV1().Secrets(serverlessNamespace).Get(svcCtx.Context, serverlessSecretName, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			logger.Infof("Secret %s not found in namespace: %s. Generating new credentials for %s", serverlessSecretName, serverlessNamespace, serverlessDockerRegistryDeploymentName)
			username, err = randAlphaNum(usernameSize)
			if err != nil {
				return errors.Wrap(err, "failed to generate username")
			}
			password, err = randAlphaNum(passwordSize)
			if err != nil {
				return errors.Wrap(err, "failed to generate password")
			}
			rollHash, err = randAlphaNum(rollMeSize)
			if err != nil {
				return errors.Wrap(err, "failed to generate rollme hash")
			}
		} else {
			return errors.Wrap(err, "failed to fetch existing serverless docker registry secret")
		}
	} else {
		logger.Infof("Secret %s found in namespace: %s. Reusing existing credentials for %s", serverlessSecretName, serverlessNamespace, serverlessDockerRegistryDeploymentName)
		username, err = readSecretKey(secret, "username")
		if err != nil {
			return errors.Wrap(err, "failed to fetch username from existing secret")
		}
		password, err = readSecretKey(secret, "password")
		if err != nil {
			return errors.Wrap(err, "failed to fetch password from existing secret")
		}
		deployment, err := k8sClient.AppsV1().Deployments(serverlessNamespace).Get(svcCtx.Context, serverlessDockerRegistryDeploymentName, metav1.GetOptions{})
		if err != nil {
			return errors.Wrap(err, "failed to fetch existing serverless docker registry deployment")
		}
		if deployment.Spec.Template.ObjectMeta.Annotations == nil {
			deployment.Spec.Template.ObjectMeta.Annotations = map[string]string{}
		}
		rollMeValue, ok := deployment.Spec.Template.ObjectMeta.Annotations["rollme"]
		if !ok {
			rollHash, err = randAlphaNum(rollMeSize)
		}
		rollHash = rollMeValue
	}

	logger.Infof("Username: %s, password: %s, rollme: %s", username, password, rollHash)

	svcCtx.Model.Configuration["serverless.dockerRegistry.username"] = username
	svcCtx.Model.Configuration["serverless.dockerRegistry.password"] = password
	svcCtx.Model.Configuration["serverless.docker-registry.rollme"] = rollHash

	return service.NewInstall(svcCtx.Logger).Invoke(svcCtx.Context, svcCtx.ChartProvider, svcCtx.Model, svcCtx.KubeClient)
}

func randAlphaNum(length int) (string, error) {
	s := make([]byte, length/2)
	if _, err := rand.Read(s); err != nil {
		return "", errors.Wrap(err, "failed to generate random key")
	}
	return fmt.Sprintf("%X", s), nil
}

func readSecretKey(secret *v1.Secret, secretKey string) (string, error) {
	if secret.Data == nil {
		return "", errors.New(fmt.Sprintf("failed to read %s from nil secret data", secretKey))
	}
	secretValue, ok := secret.Data[secretKey]
	if !ok {
		return "", errors.New(fmt.Sprintf("%s is not found in secret", secretKey))
	}
	stringValue := string(secretValue)
	if stringValue == "" {
		return "", errors.New(fmt.Sprintf("%s was found in secret but its empty", secretKey))
	}
	return stringValue, nil
}
