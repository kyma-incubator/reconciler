package ory

import (
	"context"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/ory/hydra"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/ory/k8s"
	internalKubernetes "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"

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
	oryChart        = "ory"
	oryNamespace    = "kyma-system"
	jwksAlg         = "RSA256"
	jwksBits        = 2048
	hydraDeployment = "ory-hydra"
)

type oryAction struct {
	step string
}

type preReconcileAction struct {
	*oryAction
}

type postReconcileAction struct {
	*oryAction
	hydraSyncer    hydra.Syncer
	rolloutHandler k8s.RolloutHandler
}

type preDeleteAction struct {
	*oryAction
	finderHandler k8s.OryCRDsHandler
}

type postDeleteAction struct {
	*oryAction
}

var (
	jwksNamespacedName = types.NamespacedName{Name: "ory-oathkeeper-jwks-secret", Namespace: oryNamespace}
	dbNamespacedName   = types.NamespacedName{Name: "ory-hydra-credentials", Namespace: oryNamespace}
	rolloutHydra       = false
)

func (a *postReconcileAction) Run(context *service.ActionContext) error {
	logger, kubeclient, cfg, _, err := readActionContext(context)
	if err != nil {
		return errors.Wrap(err, "Failed to read postReconcileAction context")
	}

	if rolloutHydra {
		logger.Info("Rolling out Ory Hydra")
		if err := a.rolloutHydraDeployment(context.Context, kubeclient, hydraDeployment, logger); err != nil {
			return errors.Wrap(err, "failed to roll out Hydra deployment")
		}
	}

	if isInMemoryMode(cfg) {
		logger.Debug("Detected in hydra in memory mode, triggering synchronization")
		err = a.hydraSyncer.TriggerSynchronization(context.Context, kubeclient, logger, oryNamespace, rolloutHydra)
		if err != nil {
			return errors.Wrap(err, "failed to trigger hydra sychronization")
		}
	} else {
		logger.Debug("Hydra is in persistence mode, no synchronization needed")
	}

	logger.Debugf("Action '%s' executed (passed version was '%s')", a.step, context.Task.Version)

	return nil
}

func (a *preReconcileAction) Run(context *service.ActionContext) error {
	logger, kubeClient, _, values, err := readActionContext(context)
	if err != nil {
		return errors.Wrap(err, "Failed to read preReconcileAction context")
	}
	client, err := kubeClient.Clientset()
	if err != nil {
		return errors.Wrap(err, "Failed to retrieve clientset")
	}
	_, err = getSecret(context.Context, client, jwksNamespacedName)
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return errors.Wrap(err, "Could not get JWKS secret")
		}

		logger.Info("Ory JWKS secret does not exist, creating it now")
		jwksSecretObject, err := jwks.Get(jwksNamespacedName, jwksAlg, jwksBits)
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
		logger.Debug("Ory DB secret exists, looking for differences")
		newSecretData, err := db.Update(values, dbSecretObject, logger)
		if err != nil {
			return errors.Wrap(err, "failed to update db credentials data for Ory Hydra")
		}

		if !isUpdate(newSecretData) {
			logger.Debug("Ory DB secret is the same as values, no need to update")
			rolloutHydra = false
		} else {
			logger.Info("Ory DB secret is different than values, updating it")
			dbSecretObject.StringData = newSecretData
			rolloutHydra = true

			if err := a.updateSecret(context.Context, client, dbNamespacedName, *dbSecretObject, logger); err != nil {
				return errors.Wrap(err, "failed to update Ory secret")
			}
		}
	}

	logger.Debugf("Action '%s' executed (passed version was '%s')", a.step, context.Task.Version)

	return nil
}

func (a *preDeleteAction) Run(context *service.ActionContext) error {
	logger := context.Logger
	kubeconfig := context.KubeClient.Kubeconfig()

	oryCRDs, err := a.finderHandler.FindAndDeleteOryFinalizers(kubeconfig, logger)
	if err != nil {
		logger.Errorf("failed to get ory CRDs")
	}

	logger.Debugf("Action '%s' executed (passed version was '%s')", a.step, context.Task.Version)
	return nil
}

func (a *postDeleteAction) Run(context *service.ActionContext) error {
	logger := context.Logger
	client, err := context.KubeClient.Clientset()
	if err != nil {
		return errors.Wrap(err, "failed to retrieve native Kubernetes GO client")
	}

	secretExists, err := a.secretExists(context.Context, client, dbNamespacedName)
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

	jwksSecretExists, err := a.secretExists(context.Context, client, jwksNamespacedName)
	if err != nil {
		return errors.Wrapf(err, "failed to get JWKS secret %s", jwksNamespacedName.Name)
	}
	if jwksSecretExists {
		err = a.deleteSecret(context.Context, client, jwksNamespacedName, logger)
		if err != nil {
			return errors.Wrapf(err, "failed to delete DB secret %s", jwksNamespacedName.Name)
		}
	} else {
		logger.Infof("JWKS Secret %s does not exist", jwksNamespacedName.Name)
	}

	logger.Debugf("Action '%s' executed (passed version was '%s')", a.step, context.Task.Version)
	return nil
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
	logger.Debugf("Secret %s updated", name.String())

	return err
}

func (a *postReconcileAction) rolloutHydraDeployment(ctx context.Context, client internalKubernetes.Client, deployment string, logger *zap.SugaredLogger) error {
	err := a.rolloutHandler.RolloutAndWaitForDeployment(ctx, deployment, oryNamespace, client, logger)
	if err != nil {
		return errors.Wrapf(err, "Failed to rollout %s deployment", deployment)
	}
	logger.Debugf("Performed rollout restart of %s deployment", deployment)

	return nil
}

func (a *postDeleteAction) secretExists(ctx context.Context, client kubernetes.Interface, name types.NamespacedName) (bool, error) {
	_, err := client.CoreV1().Secrets(name.Namespace).Get(ctx, name.Name, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "Could not get %s secret", name.Name)
	}
	return true, nil
}

func (a *postDeleteAction) deleteSecret(ctx context.Context, client kubernetes.Interface, name types.NamespacedName, logger *zap.SugaredLogger) error {
	err := client.CoreV1().Secrets(name.Namespace).Delete(ctx, name.Name, metav1.DeleteOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			logger.Infof("Secret %s does not exist anymore", name.String())
			return nil
		}
		return errors.Wrap(err, "failed to delete the secret")
	}
	logger.Debugf("Secret %s deleted", name.String())

	return err
}

func createSecret(ctx context.Context, client kubernetes.Interface, name types.NamespacedName, secret v1.Secret, logger *zap.SugaredLogger) error {
	_, err := client.CoreV1().Secrets(name.Namespace).Create(ctx, &secret, metav1.CreateOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to create the secret")
	}
	logger.Debugf("Secret %s created", name.String())

	return err
}

func readActionContext(context *service.ActionContext) (*zap.SugaredLogger, internalKubernetes.Client, *db.Config, map[string]interface{}, error) {
	logger := context.Logger
	component := chart.NewComponentBuilder(context.Task.Version, oryChart).
		WithNamespace(oryNamespace).
		WithProfile(context.Task.Profile).
		WithConfiguration(context.Task.Configuration).Build()

	chartValues, err := context.ChartProvider.Configuration(component)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "Failed to retrieve ory chart values")
	}
	client := context.KubeClient
	cfg, err := db.NewDBConfig(chartValues)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "Failed to retrieve native Kubernetes GO client")
	}
	return logger, client, cfg, chartValues, nil
}

func isInMemoryMode(cfg *db.Config) bool {
	return !cfg.Global.Ory.Hydra.Persistence.Enabled
}

func isUpdate(diff map[string]string) bool {
	return len(diff) != 0
}
