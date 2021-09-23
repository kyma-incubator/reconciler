package data

import (
	"context"
	"strings"

	"github.com/avast/retry-go"
	"go.uber.org/zap"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

//go:generate mockery -name=Gatherer -outpkg=mocks -case=underscore
// Gatherer gathers data from the Kubernetes cluster.
type Gatherer interface {
	GetAllPods() (podsList *v1.PodList, err error)
	GetPodsWithDifferentImage(inputPodsList v1.PodList, image ExpectedImage) (outputPodsList v1.PodList)
}

// DefaultGatherer that gets pods from the Kubernetes cluster
type DefaultGatherer struct {
	kubeClient kubernetes.Interface
	retryOpts  []retry.Option
	log        *zap.SugaredLogger
}

// ExpectedImage to be verified by the proxy.
type ExpectedImage struct {
	Prefix  string
	Version string
}

func NewDefaultGatherer(kubeConfigPath string, retryOpts []retry.Option, log *zap.SugaredLogger) (*DefaultGatherer, error) {
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return nil, err
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return &DefaultGatherer{kubeClient: kubeClient, retryOpts: retryOpts, log: log}, nil
}

func (i *DefaultGatherer) GetAllPods() (podsList *v1.PodList, err error) {
	err = retry.Do(func() error {
		podsList, err = i.kubeClient.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return err
		}

		return nil
	}, i.retryOpts...)

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
			hasPrefix := strings.HasPrefix(container.Image, image.Prefix)
			hasSuffix := strings.HasSuffix(container.Image, image.Version)
			isTerminating := pod.Status.Phase == "Terminating"

			if hasPrefix && !hasSuffix && !isTerminating {
				outputPodsList.Items = append(outputPodsList.Items, *pod.DeepCopy())
			}
		}
	}

	return
}
