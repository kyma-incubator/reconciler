package ory

import (
	"context"
	"fmt"
	"time"

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

type oryAction struct {
	step string
}

type preReconcileAction struct {
	*oryAction
}

type preDeleteAction struct {
	*oryAction
}

var (
	jwksNamespacedName = types.NamespacedName{Name: "ory-oathkeeper-jwks-secret", Namespace: oryNamespace}
	dbNamespacedName   = types.NamespacedName{Name: "ory-hydra-credentials", Namespace: oryNamespace}
)

func (a *preReconcileAction) Run(context *service.ActionContext) error {
	logger := context.Logger
	component := chart.NewComponentBuilder(context.Task.Version, oryChart).
		WithNamespace(oryNamespace).
		WithProfile(context.Task.Profile).
		WithConfiguration(context.Task.Configuration).Build()

	values, err := context.ChartProvider.Configuration(component)
	if err != nil {
		return errors.Wrap(err, "failed to retrieve Ory chart values")
	}

	client, err := context.KubeClient.Clientset()
	if err != nil {
		return errors.Wrap(err, "failed to retrieve native Kubernetes GO client")
	}

	jwksSecretObject, err := getSecret(context.Context, client, jwksNamespacedName)
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return errors.Wrap(err, "Could not get JWKS secret")
		}

		logger.Info("Ory JWKS secret does not exist, creating it now")
		jwksSecretObject, err = jwks.Get(jwksNamespacedName, jwksAlg, jwksBits)
		if err != nil {
			return errors.Wrap(err, "failed to create jwks secret for ORY OathKeeper")
		}
		if err := createSecret(context.Context, client, jwksNamespacedName, *jwksSecretObject, logger); err != nil {
			return err
		}
	}

	dbSecretObject, err := getSecret(context.Context, client, dbNamespacedName)
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return errors.Wrap(err, "Could not get DB secret")
		}

		logger.Info("Ory DB secret does not exist, creating it now")
		dbSecretObject, err = db.Get(dbNamespacedName, values, logger)
		if err != nil {
			return errors.Wrap(err, "failed to create db credentials data for Ory Hydra")
		}
		if err := createSecret(context.Context, client, dbNamespacedName, *dbSecretObject, logger); err != nil {
			return err
		}

	} else {
		logger.Info("Ory DB secret exists, looking for differences")
		newSecretData, err := db.Update(values, dbSecretObject, logger)
		if err != nil {
			return errors.Wrap(err, "failed to update db credentials data for Ory Hydra")
		}

		if len(newSecretData) == 0 {
			logger.Info("Ory DB secret is the same as values, no need to update")
		} else {
			logger.Info("Ory DB secret is different than values, updating it")
			dbSecretObject.StringData = newSecretData

			if err := a.updateSecret(context.Context, client, dbNamespacedName, *dbSecretObject, logger); err != nil {
				return errors.Wrap(err, "failed to update Ory secret")
			}
			logger.Info("Rolling out ory hydra")
			if err := a.rolloutHydraDeployment(context.Context, client, logger); err != nil {
				return err
			}
		}
	}

	logger.Infof("Action '%s' executed (passed version was '%s')", a.step, context.Task.Version)

	return nil
}

func (a *preDeleteAction) Run(context *service.ActionContext) error {
	logger := context.Logger
	client, err := context.KubeClient.Clientset()
	if err != nil {
		return errors.Wrap(err, "failed to retrieve native Kubernetes GO client")
	}

	secretExists, err := a.dbSecretExists(context.Context, client, dbNamespacedName)
	if err != nil {
		return errors.Wrapf(err, "failed to get DB secret %s", dbNamespacedName.Name)
	}
	if secretExists {
		err = a.deleteSecret(context.Context, client, dbNamespacedName, logger)
		if err != nil {
			return errors.Wrapf(err, "failed to delete DB secret %s", dbNamespacedName.Name)
		}
	} else {
		logger.Infof("DB Secret %s does not exist", dbNamespacedName.Name)
	}

	logger.Infof("Action '%s' executed (passed version was '%s')", a.step, context.Task.Version)
	return nil
}

func isEmpty(secret *v1.Secret) bool {
	return len(secret.Data) == 0
}

func getSecret(ctx context.Context, client kubernetes.Interface, name types.NamespacedName) (*v1.Secret, error) {
	secret, err := client.CoreV1().Secrets(name.Namespace).Get(ctx, name.Name, metav1.GetOptions{})
	if err != nil {
		return secret, errors.Wrap(err, "failed to get Ory secret")
	}

	return secret, err
}

func (a *preReconcileAction) updateSecret(ctx context.Context, client kubernetes.Interface, name types.NamespacedName, secret v1.Secret, logger *zap.SugaredLogger) error {
	_, err := client.CoreV1().Secrets(name.Namespace).Update(ctx, &secret, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to update the secret")
	}
	logger.Infof("Secret %s updated", name.String())

	return err
}

func (a *preReconcileAction) rolloutHydraDeployment(ctx context.Context, client kubernetes.Interface, logger *zap.SugaredLogger) error {
	data := fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"%s"}}}}}`, time.Now().String())

	_, err := client.AppsV1().Deployments("kyma-system").Patch(ctx, "ory-hydra", types.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
	if err != nil {
		return errors.Wrap(err, "Failed to rollout ory hydra")
	}
	logger.Info("ory-hydra restarted")

	return nil
}

func (a *preDeleteAction) dbSecretExists(ctx context.Context, client kubernetes.Interface, name types.NamespacedName) (bool, error) {
	_, err := client.CoreV1().Secrets(name.Namespace).Get(ctx, name.Name, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrap(err, "Could not get DB secret")
	}
	return true, nil
}

func (a *preDeleteAction) deleteSecret(ctx context.Context, client kubernetes.Interface, name types.NamespacedName, logger *zap.SugaredLogger) error {
	err := client.CoreV1().Secrets(name.Namespace).Delete(ctx, name.Name, metav1.DeleteOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			logger.Infof("Secret %s does not exist anymore", name.String())
			return nil
		}
		return errors.Wrap(err, "failed to delete the secret")
	}
	logger.Infof("Secret %s deleted", name.String())

	return err
}

func createSecret(ctx context.Context, client kubernetes.Interface, name types.NamespacedName, secret v1.Secret, logger *zap.SugaredLogger) error {
	_, err := client.CoreV1().Secrets(name.Namespace).Create(ctx, &secret, metav1.CreateOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to create the secret")
	}
	logger.Infof("Secret %s created", name.String())

	return err
}
