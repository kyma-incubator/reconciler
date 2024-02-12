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
const containerName = "admission"

type CleanupWardenAdmissionCertColumeMounts struct {
	name string
}

func (a *CleanupWardenAdmissionCertColumeMounts) Run(context *service.ActionContext) error {

	context.Logger.Infof("Action '%s' executed (passed version was '%s')", a.name, context.Task.Version)

	k8sClient := context.KubeClient

	targetImage := getWardenAdmissionTargetImage(context.Task.Configuration)

	context.Logger.Infof("target image %s", targetImage)

	if isQualifiedForCleanup(targetImage) {

		deployment, err := getDeployment(context.Context, k8sClient, wardenAdmissionDeploymentName, wardenAdmissionDeploymentNamespace)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("while checking if %s deployment is present on cluster", wardenAdmissionDeploymentName))
		}
		if deployment != nil {

			wardenContainerIndex := getContainerIndexByName(deployment, containerName)
			if wardenContainerIndex == -1 {
				context.Logger.Infof("no action needed for warden admission deployment before applying image %s as no container with name %s was found", targetImage, containerName)
				return nil
			}
			volumeIndex := getVolumeIndexByName(deployment, volumeName)
			volumeMountIndex := getVolumeMountIndexByName(deployment, wardenContainerIndex, volumeName)

			if volumeIndex == -1 || volumeMountIndex == -1 {
				context.Logger.Infof("no action needed for warden admission deployment before applying image %s as no certs volumes were found", targetImage)
				return nil
			}

			context.Logger.Infof("warden admission deployment qualifies for Volume[%d] nad VolumeMount[%d] cleanup before applying image %s", volumeIndex, volumeMountIndex, targetImage)
			data := fmt.Sprintf(`[{"op": "remove", "path": "/spec/template/spec/containers/%d/volumeMounts/%d"},{"op": "remove", "path": "/spec/template/spec/volumes/%d"}]`, wardenContainerIndex, volumeMountIndex, volumeIndex)
			err = k8sClient.PatchUsingStrategy(context.Context, "Deployment", wardenAdmissionDeploymentName, wardenAdmissionDeploymentNamespace, []byte(data), types.StrategicMergePatchType)
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("while patching  %s deployment", wardenAdmissionDeploymentName))
			}
		}
	}

	context.Logger.Infof("no action required for new admission image [\"%s\"]", targetImage)
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

func getVolumeMountIndexByName(deployment *appsv1.Deployment, containerIndex int, volumeMountName string) int {
	for p, v := range deployment.Spec.Template.Spec.Containers[containerIndex].VolumeMounts {
		if v.Name == volumeMountName {
			return p
		}
	}
	return -1
}

func getContainerIndexByName(deployment *appsv1.Deployment, containerName string) int {
	for p, c := range deployment.Spec.Template.Spec.Containers {
		if c.Name == containerName {
			return p
		}
	}
	return -1
}

func getWardenAdmissionTargetImage(config map[string]interface{}) string {
	val, ok := config["global.admission.image"]
	if !ok {
		return ""
	}
	stringValue, ok := val.(string)
	if !ok {
		return ""
	}
	return stringValue
}
