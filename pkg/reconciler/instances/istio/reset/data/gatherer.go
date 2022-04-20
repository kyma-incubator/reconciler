package data

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/avast/retry-go"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//go:generate mockery --name=Gatherer --outpkg=mocks --case=underscore
// Gatherer gathers data from the Kubernetes cluster.
type Gatherer interface {
	// GetAllPods from the cluster and return them as a v1.PodList.
	GetAllPods(kubeClient kubernetes.Interface, retryOpts []retry.Option) (podsList *v1.PodList, err error)

	// GetPodsWithDifferentImage than the passed expected image to filter them out from the pods list.
	GetPodsWithDifferentImage(inputPodsList v1.PodList, image ExpectedImage) (outputPodsList v1.PodList)
}

// DefaultGatherer that gets pods from the Kubernetes cluster
type DefaultGatherer struct{}

// ExpectedImage to be verified by the proxy.
type ExpectedImage struct {
	Prefix  string
	Version string
}

// NewDefaultGatherer creates a new instance of DefaultGatherer.
func NewDefaultGatherer() *DefaultGatherer {
	return &DefaultGatherer{}
}

func (i *DefaultGatherer) GetAllPods(kubeClient kubernetes.Interface, retryOpts []retry.Option) (podsList *v1.PodList, err error) {
	err = retry.Do(func() error {
		podsList, err = kubeClient.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return err
		}

		return nil
	}, retryOpts...)

	if err != nil {
		return nil, err
	}

	return
}

func (i *DefaultGatherer) GetPodsWithDifferentImage(inputPodsList v1.PodList, image ExpectedImage) (outputPodsList v1.PodList) {
	inputPodsList.DeepCopyInto(&outputPodsList)
	outputPodsList.Items = []v1.Pod{}

	for _, pod := range inputPodsList.Items {
		if _, ok := pod.Annotations["sidecar.istio.io/status"]; !ok || !isPodReady(pod) {
			continue
		}

		type IstioStatusStruct struct {
			Containers []string `json:"containers"`
		}
		istioStatus := IstioStatusStruct{}
		json.Unmarshal([]byte(fmt.Sprintf("{%s}", pod.Annotations["sidecar.istio.io/status"])), &istioStatus)

		for _, container := range pod.Spec.Containers {
			isIstioSidecar := false
			for _, c := range istioStatus.Containers {
				if c == container.Name {
					isIstioSidecar = true
					break
				}
			}
			containsPrefix := strings.Contains(container.Image, image.Prefix)
			hasSuffix := strings.HasSuffix(container.Image, image.Version)
			if (!hasSuffix || !containsPrefix) && isIstioSidecar {
				outputPodsList.Items = append(outputPodsList.Items, *pod.DeepCopy())
			}
		}
	}

	return
}

// isPodReady checks if the pod is Ready, returns true if the Pod is in the Running state and not Pending or Terminating.
func isPodReady(pod v1.Pod) bool {

	if pod.Status.Phase != v1.PodRunning {
		return false
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Status != v1.ConditionTrue {
			return false
		}
	}

	return pod.ObjectMeta.DeletionTimestamp == nil
}

// RemoveAnnotatedPods removes pods with annotation annotationKey from in podList
func RemoveAnnotatedPods(in v1.PodList, annotationKey string) (out v1.PodList) {
	in.DeepCopyInto(&out)
	out.Items = []v1.Pod{}
	for i := 0; i < len(in.Items); i++ {
		if _, ok := in.Items[i].Annotations[annotationKey]; !ok {
			out.Items = append(out.Items, in.Items[i])
		}
	}
	return
}
