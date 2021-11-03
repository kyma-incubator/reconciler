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

type preAction struct {
	*oryAction
}

type postAction struct {
	*oryAction
}

var (
	jwksNamespacedName = types.NamespacedName{Name: "ory-oathkeeper-jwks-secret", Namespace: oryNamespace}
	dbNamespacedName   = types.NamespacedName{Name: "ory-hydra-credentials", Namespace: oryNamespace}
)

func (a *preAction) Run(context *service.ActionContext) error {
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

	secretObject, err := a.getDBConfigSecret(context.Context, client, dbNamespacedName)
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return errors.Wrap(err, "Could not get DB secret")
		}

		logger.Info("Ory DB secret does not exist, creating it now")
		secretObject, err = db.Get(dbNamespacedName, values, logger)
		if err != nil {
			return errors.Wrap(err, "failed to create db credentials data for Ory Hydra")
		}
		if err := a.ensureOrySecret(context.Context, client, dbNamespacedName, *secretObject, logger); err != nil {
			return errors.Wrap(err, "failed to ensure Ory secret")
		}

	} else {
		logger.Info("Ory DB secret exists, looking for differences")
		isUpdate, err := db.IsUpdate(dbNamespacedName, values, secretObject, logger)
		if err != nil {
			return errors.Wrap(err, "failed to update db credentials data for Ory Hydra")
		}
		if !isUpdate {
			logger.Info("Ory DB secret is the same as values, no need to update")
		} else {
			logger.Info("Ory DB secret is different than values, updating it")
			if err := a.updateSecret(context.Context, client, dbNamespacedName, *secretObject, logger); err != nil {
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

func (a *postAction) Run(context *service.ActionContext) error {
	logger := context.Logger
	client, err := context.KubeClient.Clientset()
	if err != nil {
		return errors.Wrap(err, "failed to retrieve native Kubernetes GO client")
	}
	patchData, err := jwks.Get(jwksAlg, jwksBits)
	if err != nil {
		return errors.Wrap(err, "failed to generate JWKS secret")
	}
	if err := a.patchSecret(context.Context, client, jwksNamespacedName, patchData, logger); err != nil {
		return errors.Wrap(err, "failed to patch Ory secret")
	}

	logger.Infof("Action '%s' executed (passed version was '%s')", a.step, context.Task.Version)

	return nil
}

func (a *preAction) getDBConfigSecret(ctx context.Context, client kubernetes.Interface, name types.NamespacedName) (*v1.Secret, error) {
	secret, err := client.CoreV1().Secrets(name.Namespace).Get(ctx, name.Name, metav1.GetOptions{})
	if err != nil {
		return secret, errors.Wrap(err, "failed to get Ory DB secret")
	}

	return secret, err
}

func (a *preAction) ensureOrySecret(ctx context.Context, client kubernetes.Interface, name types.NamespacedName, secret v1.Secret, logger *zap.SugaredLogger) error {
	_, err := client.CoreV1().Secrets(name.Namespace).Get(ctx, name.Name, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return createSecret(ctx, client, name, secret, logger)
		}
		return errors.Wrap(err, "failed to get Ory DB secret")
	}

	return err
}

func (a *preAction) updateSecret(ctx context.Context, client kubernetes.Interface, name types.NamespacedName, secret v1.Secret, logger *zap.SugaredLogger) error {
	_, err := client.CoreV1().Secrets(name.Namespace).Update(ctx, &secret, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to update the secret")
	}
	logger.Infof("Secret %s updated", name.String())
	return err
}

func (a *preAction) rolloutHydraDeployment(ctx context.Context, client kubernetes.Interface, logger *zap.SugaredLogger) error {
	data := fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"%s"}}}}}`, time.Now().String())

	_, err := client.AppsV1().Deployments("kyma-system").Patch(ctx, "ory-hydra", types.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
	if err != nil {
		return errors.Wrap(err, "Failed to rollout ory hydra")
	}

	return nil
}

func (a *postAction) patchSecret(ctx context.Context, client kubernetes.Interface, name types.NamespacedName, data []byte, logger *zap.SugaredLogger) error {
	_, err := client.CoreV1().Secrets(name.Namespace).Get(ctx, name.Name, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to get secret")
	}
	_, err = client.CoreV1().Secrets(name.Namespace).Patch(ctx, name.Name, types.JSONPatchType, data, metav1.PatchOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to patch the secret")
	}
	logger.Infof("Secret %s patched", name.String())

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
