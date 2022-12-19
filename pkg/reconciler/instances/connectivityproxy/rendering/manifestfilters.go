package rendering

import (
	"github.com/coreos/go-semver/semver"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	ReleaseLabelKey       = "release"
	ConnectivityProxyKind = "StatefulSet"
)

func NewFilterOutAnnotatedManifests(annotation string) FilterFunc {
	return func(unstructs []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
		newUnstructs := make([]*unstructured.Unstructured, 0)

		for _, unstruct := range unstructs {
			annotations := unstruct.GetAnnotations()
			_, ok := annotations[annotation]
			if !ok {
				newUnstructs = append(newUnstructs, unstruct)
			}
		}

		return newUnstructs, nil
	}
}

func NewSkipReinstallingCurrentRelease(logger *zap.SugaredLogger, appName, currentRelease string) FilterFunc {
	return func(unstructs []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
		var statefulSetManifest *unstructured.Unstructured
		for _, unstruct := range unstructs {
			if unstruct != nil && unstruct.GetName() == appName && unstruct.GetKind() == ConnectivityProxyKind {
				statefulSetManifest = unstruct
				break
			}
		}

		if statefulSetManifest == nil {
			logger.Warn("Connectivity Proxy StatefulSet is missing in the chart")
			return []*unstructured.Unstructured{}, errors.Errorf("Connectivity Proxy stateful set does not have any release labels")
		}

		releaseToBeInstalled := ""
		if statefulSetManifest.GetLabels() != nil {
			releaseToBeInstalled = statefulSetManifest.GetLabels()[ReleaseLabelKey]
		}

		if releaseToBeInstalled == "" {
			return []*unstructured.Unstructured{}, errors.Errorf("Connectivity Proxy StatefulSet does not have any release labels")
		}

		currentVersion, err := semver.NewVersion(currentRelease)
		if err != nil {
			return []*unstructured.Unstructured{}, errors.Errorf("incorrect release version format: %s", currentRelease)
		}

		newVersion, err := semver.NewVersion(releaseToBeInstalled)
		if err != nil {
			return []*unstructured.Unstructured{}, errors.Errorf("incorrect release version format: %s", releaseToBeInstalled)
		}

		if currentVersion.LessThan(*newVersion) {
			logger.Info("Connectivity Proxy release has changed, the component will be upgraded")
			return unstructs, nil
		}

		logger.Debug("Connectivity Proxy release did not change, skipping")
		return []*unstructured.Unstructured{}, nil
	}
}
