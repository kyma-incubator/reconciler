package service

import (
	"context"
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type Install struct {
	logger *zap.SugaredLogger
}

func NewInstall(logger *zap.SugaredLogger) *Install {
	return &Install{logger: logger}
}

//go:generate mockery --name=Operation --output=mocks --outpkg=mocks --case=underscore
type Operation interface {
	Invoke(ctx context.Context, chartProvider chart.Provider, model *reconciler.Task, kubeClient kubernetes.Client) error
}

func (r *Install) Invoke(ctx context.Context, chartProvider chart.Provider, task *reconciler.Task, kubeClient kubernetes.Client) error {
	var err error
	var manifest string
	if task.Component == model.CRDComponent {
		manifest, err = r.renderCRDs(chartProvider, task)
	} else {
		manifest, err = r.renderManifest(chartProvider, task)
	}
	if err != nil {
		return err
	}

	if task.Type == model.OperationTypeDelete {
		resources, err := kubeClient.Delete(ctx, manifest, task.Namespace)
		if err == nil {
			r.logger.Debugf("Deletion of manifest finished successfully: %d resources deleted", len(resources))
		} else {
			r.logger.Warnf("Failed to delete manifests on target cluster: %s", err)
			return err
		}
	} else {
		if task.Type == model.OperationTypeReconcile && task.Component == model.CleanupComponent {
			return nil
		}
		resources, err := kubeClient.Deploy(ctx, manifest, task.Namespace,
			&LabelsInterceptor{
				Version: task.Version,
			},
			&AnnotationsInterceptor{},
			&ServicesInterceptor{
				kubeClient: kubeClient,
			},
		)
		if err == nil {
			r.logger.Debugf("Deployment of manifest finished successfully: %d resources deployed", len(resources))
		} else {
			r.logger.Warnf("Failed to deploy manifests on target cluster: %s", err)
			return err
		}
	}
	return nil
}

func (r *Install) renderManifest(chartProvider chart.Provider, model *reconciler.Task) (string, error) {
	component := chart.NewComponentBuilder(model.Version, model.Component).
		WithProfile(model.Profile).
		WithNamespace(model.Namespace).
		WithConfiguration(model.Configuration).
		WithURL(model.URL).
		Build()

	//get manifest of component
	chartManifest, err := chartProvider.RenderManifest(component)
	if err != nil {
		msg := fmt.Sprintf("Failed to get manifest for component '%s' in Kyma version '%s'",
			model.Component, model.Version)
		if model.URL != "" {
			msg += fmt.Sprintf(" using repository '%s' ",
				model.URL)
		}
		r.logger.Errorf("%s: %s", msg, err)
		return "", errors.Wrap(err, msg)
	}

	return chartManifest.Manifest, nil
}

func (r *Install) renderCRDs(chartProvider chart.Provider, model *reconciler.Task) (string, error) {
	crdManifests, err := chartProvider.RenderCRD(model.Version)
	if err != nil {
		msg := fmt.Sprintf("Failed to get CRD manifests for Kyma version '%s'", model.Version)
		r.logger.Errorf("%s: %s", msg, err)
		return "", errors.Wrap(err, msg)
	}
	return chart.MergeManifests(crdManifests...), nil
}
