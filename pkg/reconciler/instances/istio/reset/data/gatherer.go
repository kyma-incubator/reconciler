package data

import (
	"context"
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
		for _, container := range pod.Spec.Containers {
			containsPrefix := strings.Contains(container.Image, image.Prefix)
			hasSuffix := strings.HasSuffix(container.Image, image.Version)
			isTerminating := pod.Status.Phase == "Terminating"

			if containsPrefix && !hasSuffix && !isTerminating {
				outputPodsList.Items = append(outputPodsList.Items, *pod.DeepCopy())
			}
		}
	}

	return
}
