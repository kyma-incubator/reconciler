package service

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
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
	} else if task.Component != model.CleanupComponent { // TODO add better support for components that do not have manifests
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
		if task.Component == model.CleanupComponent {
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
			&PVCInterceptor{
				kubeClient: kubeClient,
				logger:     r.logger,
			},
			newClusterWideResourceInterceptor(),
			&NamespaceInterceptor{},
			&FinalizerInterceptor{
				kubeClient: kubeClient,
				interceptableKinds: []string{
					"LogPipeline",
					"LogParser",
					"OAuth2Client",
				},
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
	var crdManifests []*chart.Manifest
	var err error
	var skippedComps = r.skippedComps(model)
	if r.ignoreIstioCRD(model) {
		r.logger.Infof("Istio CRDs will be ignored from reconciliation (correlation-ID: %s)", model.CorrelationID)
		skippedComps = append(skippedComps, "istio")
	}
	crdManifests, err = chartProvider.RenderCRDFiltered(model.Version, skippedComps)
	if err != nil {
		msg := fmt.Sprintf("Failed to get CRD manifests for Kyma version '%s'", model.Version)
		r.logger.Errorf("%s: %s", msg, err)
		return "", errors.Wrap(err, msg)
	}
	return chart.MergeManifests(crdManifests...), nil
}

func (r *Install) ignoreIstioCRD(task *reconciler.Task) bool {
	restConfig, err := clientcmd.NewClientConfigFromBytes([]byte(task.Kubeconfig))
	if err != nil {
		r.logger.Warnf("Failed to create K8s REST client to check for migrated Istio CRD: %s", err)
		return true
	}

	clientConfig, err := restConfig.ClientConfig()
	if err != nil {
		r.logger.Warnf("Failed to create K8s client config to check for migrated Istio CRD: %s", err)
		return true
	}

	kubeClient, err := dynamic.NewForConfig(clientConfig)
	if err != nil {
		r.logger.Warnf("Failed to create K8s kube client to check for migrated Istio CRD: %s", err)
		return true
	}

	//check which of the configured CRDs exist on the cluster
	group := "operator.kyma-project.io"
	version := "v1alpha2"
	kind := "istios"
	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: kind,
	}
	_, err = kubeClient.Resource(gvr).List(context.Background(), v1.ListOptions{})
	if err == nil {
		r.logger.Infof("Found obviously migrated Istio CRD '%s.%s:%s'", kind, group, version)
		return true
	} else if !k8serr.IsNotFound(err) {
		r.logger.Errorf("Failed to retrieve CRD '%s.%s:%s': %s", kind, group, version, err)
		return true
	}

	return false
}

func (r *Install) skippedComps(task *reconciler.Task) []string {
	envVars := os.Environ()
	skippedComps := []string{}
	//Search for skipped components by checking all env-vars
	for _, envVar := range envVars {
		envPair := strings.SplitN(envVar, "=", 2)
		//extract the component name from the env-var and append it to the slice of skippedComps
		if strings.HasPrefix(envPair[0], model.SkippedComponentEnvVarPrefix) && (envPair[1] == "1" || strings.ToLower(envPair[1]) == "true") {
			compNameRaw := strings.Replace(envPair[0], model.SkippedComponentEnvVarPrefix, "", 1)
			compName := strings.ToLower(strings.ReplaceAll(compNameRaw, "_", "-"))
			skippedComps = append(skippedComps, compName)
			r.logger.Infof("%s CRDs will be ignored from reconciliation (skipped by env-var, correlation-ID: %s)", compName, task.CorrelationID)
		}
	}
	return skippedComps
}
