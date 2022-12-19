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
			return []*unstructured.Unstructured{}, errors.Errorf("Connectivity proxy statefulSet is missing in the chart")
		}

		releaseToBeInstalled := ""
		if statefulSetManifest.GetLabels() != nil {
			releaseToBeInstalled = statefulSetManifest.GetLabels()[ReleaseLabelKey]
		}

		if releaseToBeInstalled == "" {
			return []*unstructured.Unstructured{}, errors.Errorf("Connectivity Proxy StatefulSet does not have any release labels")
		}

		currentVersion, err := convertVersion(currentRelease)
		if err != nil {
			return []*unstructured.Unstructured{}, err
		}

		newVersion, err := convertVersion(releaseToBeInstalled)
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

func convertVersion(connectivityProxyVersion string) (*semver.Version, error) {
	version, err := semver.NewVersion(strings.Replace(connectivityProxyVersion, ReleasePrefix, "", 1))
	if err != nil {
		return nil, errors.Errorf("incorrect release version format: %s", connectivityProxyVersion)
	}

	return version, nil
}
