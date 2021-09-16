package ory

import (
	"context"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/ory/db"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/ory/jwks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

const (
	oryChart     = "ory"
	oryNamespace = "kyma-system"
	jwksAlg      = "RSA256"
	jwksBits     = 2048
)

type ReconcileAction struct {
	step string
}

var (
	jwksNamespacedName = types.NamespacedName{Name: "ory-oathkeeper-jwks-secret", Namespace: oryNamespace}
	dbNamespacedName   = types.NamespacedName{Name: "ory-hydra-credentials", Namespace: oryNamespace}
)

func (a *ReconcileAction) Run(version, profile string, config []reconciler.Configuration, context *service.ActionContext) error {
	logger := context.Logger
	component := chart.NewComponentBuilder(version, oryChart).WithNamespace(oryNamespace).WithProfile(profile).WithConfiguration(config).Build()
	values, err := context.ChartProvider.Configuration(component)
	if err != nil {
		return errors.Wrap(err, "failed to retrieve Ory chart values")
	}

	kubeClient, err := context.KubeClient.Clientset()
	if err != nil {
		return errors.Wrap(err, "failed to retrieve native Kubernetes GO client")
	}

	switch a.step {
	case "pre-install":
		secretObject, err := db.PrepareSecret(dbNamespacedName, values)
		if err != nil {
			return errors.Wrap(err, "failed to prepare db credentials data for Ory Hydra")
		}
		if err := a.ensureOrySecret(context.Context, kubeClient, dbNamespacedName, *secretObject, logger); err != nil {
			return errors.Wrap(err, "failed to ensure Ory secret")
		}
	case "post-install":
		patchData, err := jwks.PreparePatchData(jwksAlg, jwksBits)
		if err != nil {
			return errors.Wrap(err, "failed to generate JWKS secret")
		}
		if err := patchSecret(context.Context, kubeClient, jwksNamespacedName, patchData, logger); err != nil {
			return errors.Wrap(err, "failed to patch Ory secret")
		}
	}

	context.Logger.Infof("Action '%s' executed (passed version was '%s')", a.step, version)

	return nil
}

func (a *ReconcileAction) ensureOrySecret(ctx context.Context, client kubernetes.Interface, name types.NamespacedName, secret v1.Secret, logger *zap.SugaredLogger) error {
	_, err := client.CoreV1().Secrets(name.Namespace).Get(ctx, name.Name, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return createSecret(ctx, client, name, secret, logger)
		}
		return errors.Wrap(err, "failed to get secret")
	}

	return nil
}
func patchSecret(ctx context.Context, client kubernetes.Interface, name types.NamespacedName, data []byte, logger *zap.SugaredLogger) error {
	_, err := client.CoreV1().Secrets(name.Namespace).Patch(ctx, name.Name, types.JSONPatchType, data, metav1.PatchOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to patch the secret")
	}
	logger.Infof("Secret %s patched", name.String())

	return nil
}

// createSecret creates a new Ory hydra credentials secret for accessing the database.
func createSecret(ctx context.Context, client kubernetes.Interface, name types.NamespacedName, secret v1.Secret, logger *zap.SugaredLogger) error {
	_, err := client.CoreV1().Secrets(name.Namespace).Create(ctx, &secret, metav1.CreateOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to create the secret")
	}
	logger.Infof("Secret %s created", name.String())

	return err
}
