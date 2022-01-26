package cleaner

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/cleaner/pkg/cleanup"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	deleteStrategyConfigKey = "delete_strategy"
)

type CleanupAction struct {
	name string
}

func (a *CleanupAction) Run(context *service.ActionContext) error {
	if context.Task.Type != model.OperationTypeDelete {
		context.Logger.Infof("Skipping execution. This reconciler only supports 'delete' task type, but was invoked with '%s' task type", context.Task.Type)
		return nil
	}

	context.Logger.Infof("Action '%s' executed: passed version was '%s', passed type was %s", a.name, context.Task.Version, context.Task.Type)

	if _, err := context.KubeClient.Clientset(); err != nil { //cleaner how to retrieve native Kubernetes GO client
		return err
	}

	namespaces := []string{"kyma-system", "kyma-integration"}

	var kymaCRDsFinder cleanup.KymaCRDsFinder = func() ([]schema.GroupVersionResource, error) {
		crdManifests, err := context.ChartProvider.RenderCRD(context.Task.Version)
		if err != nil {
			return nil, err
		}
		return findKymaCRDs(crdManifests, context.Logger)
	}

	dropFinalizersOnlyForKymaCRs := readDeleteStrategy(context.Task.Configuration) != "all"
	cliCleaner, err := cleanup.NewCliCleaner(context.Task.Kubeconfig, namespaces, context.Logger, dropFinalizersOnlyForKymaCRs, kymaCRDsFinder)
	if err != nil {
		return err
	}

	return cliCleaner.Run()
}

func findKymaCRDs(crdManifests []*chart.Manifest, logger *zap.SugaredLogger) ([]schema.GroupVersionResource, error) {
	res := []schema.GroupVersionResource{}

	for _, crdManifest := range crdManifests {
		unstructs, _ := kubernetes.ToUnstructured([]byte(crdManifest.Manifest), true)
		for _, crdUnstruct := range unstructs {

			crdName := crdUnstruct.GetName()
			crdObject := crdUnstruct.Object

			group, ok, err := unstructured.NestedString(crdObject, "spec", "group")
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, fmt.Errorf("Error getting attribute \"spec.group\" for %s CRD", crdName)
			}

			namesPlural, ok, err := unstructured.NestedString(crdObject, "spec", "names", "plural")
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, fmt.Errorf("Error getting attribute \"spec.names.plural\" for %s CRD", crdName)
			}

			versions, versionsFound, err := unstructured.NestedSlice(crdObject, "spec", "versions")
			if err != nil {
				return nil, err
			}
			if versionsFound {
				for i, v := range versions {
					version, ok := v.(map[string]interface{})
					if !ok {
						return nil, fmt.Errorf("Error converting attribute \"spec.versions[%d]\" to map for %s CRD", i, crdName)
					}

					versionName, ok, err := unstructured.NestedString(version, "name")
					if err != nil {
						return nil, err
					}
					if !ok {
						return nil, fmt.Errorf("Error getting attribute \"spec.versions[%d].name\" for %s CRD", i, crdName)
					}
					grv := toGRV(group, versionName, namesPlural)
					logger.Debugf("Found Kyma CRD: %s.%s/%s", grv.Resource, grv.Group, grv.Version)
					res = append(res, grv)
				}
			} else {
				//No "spec.versions" attribute, look for "spec.version"
				versionName, versionOK, err := unstructured.NestedString(crdObject, "spec", "version") //deprecated attribute existing in `apiextensions.k8s.io/v1beta1`
				if err != nil {
					return nil, err
				}
				if !versionOK {
					return nil, fmt.Errorf("Can't find neither \"spec.versions\" nor \"spec.version\" for %s CRD", crdName)
				}
				grv := toGRV(group, versionName, namesPlural)
				logger.Debugf("Found Kyma CRD: %s.%s/%s", grv.Resource, grv.Group, grv.Version)
				res = append(res, grv)
			}
		}
	}

	return res, nil
}

func toGRV(group, version, resource string) schema.GroupVersionResource {

	return schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}
}

func readDeleteStrategy(config map[string]interface{}) string {
	v := config[deleteStrategyConfigKey]
	if v == nil {
		return ""
	}

	s, ok := v.(string)
	if !ok {
		return ""
	}

	return s
}
