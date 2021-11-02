package rafter

import (
	"context"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientgo "k8s.io/client-go/kubernetes"
)

const (
	rafterNamespace          = "kyma-system"
	rafterValuesRelativePath = "resources/rafter/charts/controller-manager/charts/minio/values.yaml"
	rafterSecretName         = "rafter-minio"
	accessKeySize            = 20
	secretKeySize            = 40

	accessKeyName = "accesskey"
	secretKeyName = "secretkey"
)

type CustomAction struct {
	name string
}

type rafterValues struct {
	AccessKey      string `yaml:"accessKey"`
	SecretKey      string `yaml:"secretKey"`
	ExistingSecret string `yaml:"existingSecret"`
}

func (a *CustomAction) Run(svcCtx *service.ActionContext) error {
	values, err := readRafterControllerValues(svcCtx, svcCtx.Task.Version)
	if err != nil {
		return errors.Wrap(err, "failed to read Rafter controller `values.yaml` file")
	}
	if values.ExistingSecret != "" {
		svcCtx.Logger.Infof("Rafter is using existing secret '%s', no need to apply action '%s'", values.ExistingSecret, a.name)
		return nil
	}
	kubeClient, err := svcCtx.KubeClient.Clientset()
	if err != nil {
		return errors.Wrap(err, "failed to retrieve native Kubernetes GO client")
	}

	if err := a.ensureRafterSecret(svcCtx.Context, kubeClient, values); err != nil {
		return errors.Wrap(err, "failed to ensure Rafter secret")
	}
	svcCtx.Logger.Infof("Action '%s' executed (passed version was '%s')", a.name, svcCtx.Task.Version)

	return nil
}

func (a *CustomAction) ensureRafterSecret(ctx context.Context, kubeClient clientgo.Interface, values *rafterValues) error {
	_, err := kubeClient.CoreV1().Secrets(rafterNamespace).Get(ctx, rafterSecretName, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return createRafterSecret(ctx, rafterSecretName, values, kubeClient)
		}
		return errors.Wrap(err, "failed to get secret")
	}

	return nil
}

func createRafterSecret(ctx context.Context, secretName string, values *rafterValues, kubeClient clientgo.Interface) error {
	if values == nil {
		return errors.New("rafter values is not set")
	}
	var err error
	accessKey := values.AccessKey
	secretKey := values.SecretKey

	if values.ExistingSecret != "" {
		return nil
	}
	if accessKey == "" {
		if accessKey, err = randAlphaNum(accessKeySize); err != nil {
			return errors.Wrap(err, "failed to generate accessKey")
		}

	}
	if secretKey == "" {
		if secretKey, err = randAlphaNum(secretKeySize); err != nil {
			return errors.Wrap(err, "failed to generate secretKey")
		}
	}
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: rafterNamespace,
		},
		Data: map[string][]byte{
			accessKeyName: []byte(accessKey),
			secretKeyName: []byte(secretKey),
		},
	}
	_, err = kubeClient.CoreV1().Secrets(rafterNamespace).Create(ctx, &secret, metav1.CreateOptions{})
	return err
}

func readRafterControllerValues(ctx *service.ActionContext, version string) (*rafterValues, error) {
	ws, err := ctx.WorkspaceFactory.Get(version)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve Kyma workspace for rafter action")
	}
	valuesFile := filepath.Join(ws.WorkspaceDir, rafterValuesRelativePath)

	return readValues(valuesFile)

}

func readValues(valuesFile string) (*rafterValues, error) {
	valuesYAML, err := ioutil.ReadFile(valuesFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read Rafter controller `values.yaml` file")
	}

	values := &rafterValues{}
	if err := yaml.Unmarshal(valuesYAML, &values); err != nil {
		return nil, errors.Wrap(err, "failed to parse Rafter controller `values.yaml` file")

	}
	return values, nil
}

func randAlphaNum(length int) (string, error) {
	s := make([]byte, length/2)
	if _, err := rand.Read(s); err != nil {
		return "", errors.Wrap(err, "failed to generate random key")
	}
	return fmt.Sprintf("%X", s), nil
}
