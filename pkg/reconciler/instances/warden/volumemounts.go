package warden

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

const wardenAdmissionDeploymentName = "warden-admission"
const wardenAdmissionDeploymentNamespace = "kyma-system"
const volumeName = "certs"

type CleanupWardenAdmissionCertColumeMounts struct {
	name string
}

func (a *CleanupWardenAdmissionCertColumeMounts) Run(context *service.ActionContext) error {
	k8sClient := context.KubeClient

	targetImage := getWardenAdmissionTargetImage(context.Task.Configuration)

	if isQualifiedForCleanup(targetImage) {

		deployment, err := getDeployment(context.Context, k8sClient, wardenAdmissionDeploymentName, wardenAdmissionDeploymentNamespace)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("while checking if %s deployment is present on cluster", wardenAdmissionDeploymentName))
		}
		if deployment != nil {

			volumeIndex := getVolumeIndexByName(deployment, volumeName)
			volumeMountIndex := getVolumeMountIndexByName(deployment, volumeName)

			if volumeIndex == -1 || volumeMountIndex == -1 {
				context.Logger.Debugf("no action needed for warden admission deployment before applying image %s", targetImage)
				return nil
			}

			context.Logger.Debugf("warden admission deployment qualifies for Volume[%d] nad VolumeMount[%d] cleanup before applying image %s", volumeIndex, volumeMountIndex, targetImage)
			data := fmt.Sprintf(`[{"op": "remove", "path": "/spec/template/spec/containers/0/volumeMounts/%d"},{"op": "remove", "path": "/spec/template/spec/volumes/%d"}]`, volumeMountIndex, volumeIndex)
			err = k8sClient.PatchUsingStrategy(context.Context, "Deployment", wardenAdmissionDeploymentName, wardenAdmissionDeploymentNamespace, []byte(data), types.StrategicMergePatchType)
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("while patching  %s deployment", wardenAdmissionDeploymentName))
			}
		}
	}

	context.Logger.Debugf("no action required for new admission image [\"%s\"]", targetImage)
	return nil
}

func isQualifiedForCleanup(image string) bool {
	split := strings.Split(image, ":")
	if len(split) != 2 {
		return false
	}
	return isVersionQualifiedForCleanup(split[1])
}

// Only 0.10.0 or higher versions qualify for cleanup
func isVersionQualifiedForCleanup(versionToCheck string) bool {
	version, err := semver.NewVersion(versionToCheck)
	if err != nil {
		return false //Non semver versions do not qualify for cleanup
	}
	targetVersion, _ := semver.NewVersion("0.10.0")
	return version.Compare(targetVersion) >= 0
}

func getDeployment(context context.Context, kubeClient kubernetes.Client, name, namespace string) (*appsv1.Deployment, error) {
	deployment, err := kubeClient.GetDeployment(context, name, namespace)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, errors.Wrap(err, fmt.Sprintf("while getting %s deployment", name))
	}
	return deployment, nil
}

func getVolumeIndexByName(deployment *appsv1.Deployment, volumeName string) int {
	for p, v := range deployment.Spec.Template.Spec.Volumes {
		if v.Name == volumeName {
			return p
		}
	}
	return -1
}

func getVolumeMountIndexByName(deployment *appsv1.Deployment, volumeMountName string) int {
	for p, v := range deployment.Spec.Template.Spec.Containers[0].VolumeMounts {
		if v.Name == volumeMountName {
			return p
		}
	}
	return -1
}

func getWardenAdmissionTargetImage(config map[string]interface{}) string {
	//global.admission.image
	global, ok := config["global"].(map[string]interface{})
	if !ok {
		return ""
	}
	admission, ok := global["admission"].(map[string]interface{})
	if !ok {
		return ""
	}
	image, ok := admission["image"].(string)
	if !ok {
		return ""
	}
	return image
}
