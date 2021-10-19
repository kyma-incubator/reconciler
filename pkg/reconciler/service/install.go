package service

import (
	"context"
	"fmt"

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

//go:generate mockery --name=Operation --output=mocks --outpkg=mock --case=underscore
type Operation interface {
	Invoke(ctx context.Context, chartProvider chart.Provider, model *reconciler.Reconciliation, kubeClient kubernetes.Client) error
}

func (r *Install) Invoke(ctx context.Context, chartProvider chart.Provider, model *reconciler.Reconciliation, kubeClient kubernetes.Client) error {
	var err error
	var manifest string
	if model.Component == "CRDs" {
		manifest, err = r.renderCRDs(chartProvider, model)
	} else {
		manifest, err = r.renderManifest(chartProvider, model)
	}
	if err != nil {
		return err
	}

	resources, err := kubeClient.Deploy(ctx, manifest, model.Namespace, &LabelsInterceptor{Version: model.Version}, &AnnotationsInterceptor{})

	if err == nil {
		r.logger.Debugf("Deployment of manifest finished successfully: %d resources deployed", len(resources))
	} else {
		r.logger.Warnf("Failed to deploy manifests on target cluster: %s", err)
	}

	return err
}

func (r *Install) renderManifest(chartProvider chart.Provider, model *reconciler.Reconciliation) (string, error) {
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
		r.logger.Errorf("%s: %s", msg, err)
		return "", errors.Wrap(err, msg)
	}

	return chartManifest.Manifest, nil
}

func (r *Install) renderCRDs(chartProvider chart.Provider, model *reconciler.Reconciliation) (string, error) {
	crdManifests, err := chartProvider.RenderCRD(model.Version)
	if err != nil {
		msg := fmt.Sprintf("Failed to get CRD manifests for Kyma version '%s'", model.Version)
		r.logger.Errorf("%s: %s", msg, err)
		return "", errors.Wrap(err, msg)
	}
	return chart.MergeManifests(crdManifests...), nil
}
