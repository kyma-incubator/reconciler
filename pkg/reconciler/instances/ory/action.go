package ory

import (
	"context"
	"log"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/ory/dsn"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/ory/jwks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

const (
	oryNamespace = "kyma-system"
	jwksAlg      = "RSA256"
	jwksBits     = 2048
)

type ReconcileAction struct {
	step string
}

var (
	jwksSecretNamespacedName = types.NamespacedName{Name: "ory-oathkeeper-jwks-secret", Namespace: oryNamespace}
	dsnNamespacedName        = types.NamespacedName{Name: "ory-hydra-credentials", Namespace: oryNamespace}
)

func (a *ReconcileAction) Run(version, profile string, config []reconciler.Configuration, context *service.ActionContext) error {

	kubeClient, err := context.KubeClient.Clientset()
	if err != nil {
		return errors.Wrap(err, "failed to retrieve native Kubernetes GO client")
	}
	switch a.step {
	case "pre-install":
		dsnConfig := &dsn.DBConfig{
			Enabled:      false,
			Type:         "postgres",
			Username:     "hydra",
			Password:     dsn.GenerateRandomString(10),
			URL:          "ory-postgresql.kyma-system.svc.cluster.local:5432",
			DatabaseName: "db4hydra",
		}

		secretObject := dsnConfig.PrepareSecret(dsnNamespacedName)
		if err := a.ensureOrySecret(context.Context, kubeClient, dsnNamespacedName, secretObject); err != nil {
			return errors.Wrap(err, "failed to ensure Ory secret")
		}

	case "post-install":
		patchData, err := jwks.GenerateJWKSSecret(jwksAlg, jwksBits)
		if err != nil {
			return errors.Wrap(err, "failed to generate JWKS secret")
		}
		if err := patchSecret(context.Context, kubeClient, jwksSecretNamespacedName, patchData); err != nil {
			return errors.Wrap(err, "failed to patch Ory secret")
		}
	}

	context.Logger.Infof("Action '%s' executed (passed version was '%s')", a.step, version)

	return nil
}

func (a *ReconcileAction) ensureOrySecret(ctx context.Context, client kubernetes.Interface, name types.NamespacedName, secret v1.Secret) error {
	_, err := client.CoreV1().Secrets(name.Namespace).Get(ctx, name.Name, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return createSecret(ctx, client, name, secret)
		}
		return errors.Wrap(err, "failed to get secret")
	}

	return nil
}
func patchSecret(ctx context.Context, client kubernetes.Interface, name types.NamespacedName, data []byte) error {
	_, err := client.CoreV1().Secrets(name.Namespace).Patch(ctx, name.Name, types.JSONPatchType, data, metav1.PatchOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to patch the secret")
	}

	return nil
}

// createSecret creates a new Ory hydra credentials secret for accessing the database.
func createSecret(ctx context.Context, client kubernetes.Interface, name types.NamespacedName, secret v1.Secret) error {
	_, err := client.CoreV1().Secrets(name.Namespace).Create(ctx, &secret, metav1.CreateOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to create the secret")
	}
	log.Printf("%s created", name.String())

	return err
}
