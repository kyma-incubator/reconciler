package rendering

import (
	"github.com/coreos/go-semver/semver"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"strings"
)

const (
	ReleaseLabelKey       = "release"
	ConnectivityProxyKind = "StatefulSet"
	ReleasePrefix         = "connectivity-proxy-"
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

func NewSkipReinstallingCurrentRelease(logger *zap.SugaredLogger, appName, currentReleaseLabel string) FilterFunc {
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
			return []*unstructured.Unstructured{}, errors.Errorf("Connectivity proxy statefulSet is missing in the chart")
		}

		newReleaseLabel := ""
		if statefulSetManifest.GetLabels() != nil {
			newReleaseLabel = statefulSetManifest.GetLabels()[ReleaseLabelKey]
		}

		if newReleaseLabel == "" {
			return []*unstructured.Unstructured{}, errors.Errorf("Connectivity Proxy StatefulSet does not have release label")
		}

		currentVersion, newVersion, err := convertVersions(currentReleaseLabel, newReleaseLabel)
		if err != nil {
			return []*unstructured.Unstructured{}, err
		}

		if !currentVersion.Equal(*newVersion) {
			logger.Info("Connectivity Proxy release has changed, the component will be upgraded")
			return unstructs, nil
		}

		logger.Debug("Connectivity Proxy release did not change, skipping")
		return []*unstructured.Unstructured{}, nil
	}
}

func convertVersions(current, new string) (currentVersion *semver.Version, newVersion *semver.Version, err error) {
	currentVersion, err = semver.NewVersion(strings.Replace(current, ReleasePrefix, "", -1))
	if err != nil {
		return nil, nil, errors.Errorf("incorrect release version format: %s", current)
	}

	newVersion, err = semver.NewVersion(strings.Replace(new, ReleasePrefix, "", -1))
	if err != nil {
		return nil, nil, errors.Errorf("incorrect release version format: %s", new)
	}

	return currentVersion, newVersion, nil
}
