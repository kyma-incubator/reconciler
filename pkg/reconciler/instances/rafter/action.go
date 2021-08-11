package rafter

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	rafterNamespace          = "kyma-system"
	rafterValuesRelativePath = "resources/rafter/charts/controller-manager/charts/minio/values.yaml"
	rafterSecretName         = "rafter-minio"
)

type CustomAction struct {
	name string
}

type rafterValues struct {
	AccessKey      string `yaml:"accessKey"`
	SecretKey      string `yaml:"secretKey"`
	ExistingSecret string `yaml:"existingSecret"`
}

func (a *CustomAction) Run(version, profile string, config []reconciler.Configuration, svcCtx *service.ActionContext) error {

	if err := a.ensureRafterSecret(svcCtx, version); err != nil {
		return errors.Wrap(err, "failed to ensure Rafter secret")
	}
	svcCtx.Logger.Infof("Action '%s' executed (passed version was '%s')", a.name, version)

	return nil
}

func (a *CustomAction) ensureRafterSecret(svcCtx *service.ActionContext, version string) error {
	values, err := readRafterControllerValues(svcCtx, version)
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

	_, err = kubeClient.CoreV1().Secrets(rafterNamespace).Get(svcCtx.Context, rafterSecretName, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return createRafterSecret(svcCtx.Context, rafterSecretName, values, kubeClient)
		}
		return fmt.Errorf("failed to get secret:%v", err)
	}

	return nil
}

func createRafterSecret(ctx context.Context, secretName string, values *rafterValues, kubeClient kubernetes.Interface) error {
	if values.AccessKey == "" || values.SecretKey == "" {
		return errors.New("failed to create Rafter secert. AccessKey or SecretKey are empty")
	}
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: rafterNamespace,
		},
		Data: map[string][]byte{
			"accessKey": []byte(values.AccessKey),
			"secretKey": []byte(values.SecretKey),
		},
	}
	_, err := kubeClient.CoreV1().Secrets(rafterNamespace).Create(ctx, &secret, metav1.CreateOptions{})
	return err
}

func isUsingExistingSecret(ctx context.Context, existingSecretName string, kubeClient kubernetes.Interface) (bool, error) {
	_, err := kubeClient.CoreV1().Secrets(rafterNamespace).Get(ctx, existingSecretName, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to get existing rafter secret:%v", err)
	}
	return true, nil
}

func readRafterControllerValues(ctx *service.ActionContext, version string) (*rafterValues, error) {
	ws, err := ctx.WorkspaceFactory.Get(version)
	if err != nil {
		return nil, errors.Wrap(err, "faild to retrive Kyma workspace for rafter action")
	}
	valuesFile := filepath.Join(ws.WorkspaceDir, rafterValuesRelativePath)

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
