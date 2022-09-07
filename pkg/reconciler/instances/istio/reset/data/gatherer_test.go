package data

import (
	"fmt"
	"testing"

	"github.com/avast/retry-go"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_Gatherer_GetAllPods(t *testing.T) {
	firstPod := fixPodWith("application", "kyma", "istio/proxyv2:1.10.1", "Running")
	secondPod := fixPodWith("istio", "custom", "istio/proxyv2:1.10.2", "Terminating")
	retryOpts := getTestingRetryOptions()

	t.Run("should not get any pod from the cluster when there are no pods running", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset()
		gatherer := DefaultGatherer{}

		// when
		pods, err := gatherer.GetAllPods(kubeClient, retryOpts)

		// then
		require.NoError(t, err)
		require.Empty(t, pods)
	})

	t.Run("should get all pods from the cluster when there are pods running", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(firstPod, secondPod)
		gatherer := DefaultGatherer{}

		// when
		pods, err := gatherer.GetAllPods(kubeClient, retryOpts)

		// then
		require.NoError(t, err)
		require.Len(t, pods.Items, 2)
	})
}

func Test_Gatherer_GetPodsWithDifferentImage(t *testing.T) {
	image := ExpectedImage{
		Prefix:  "istio/proxyv2",
		Version: "1.10.1",
	}

	podWithExpectedImage := fixPodWith("application", "kyma", "istio/proxyv2:1.10.1", "Running")
	podWithExpectedImageTerminating := fixPodWith("istio", "custom", "istio/proxyv2:1.10.2", "Terminating")
	podWithExpectedImagePending := fixPodWith("istio", "custom", "istio/proxyv2:1.10.2", "Pending")
	podWithDifferentImageSuffix := fixPodWith("istio", "custom", "istio/proxyv2:1.10.2", "Running")
	podWithDifferentImageSuffixTerminating := fixPodWith("application", "kyma", "istio/proxyv2:1.10.2", "Terminating")
	podWithDifferentImageSuffixPending := fixPodWith("application", "kyma", "istio/proxyv2:1.10.2", "Pending")
	podWithDifferentImagePrefix := fixPodWith("application", "kyma", "istio/someimage:1.10.2", "Running")
	podWithSoloImagePrefix := fixPodWith("application", "kyma", "istio/proxyv2-1124324:1.12.3-solo-fips", "Running")

	t.Run("should not get any pods from an empty list", func(t *testing.T) {
		// given
		var pods v1.PodList
		gatherer := DefaultGatherer{}

		// when
		podsWithDifferentImage := gatherer.GetPodsWithDifferentImage(pods, image)

		// then
		require.Empty(t, podsWithDifferentImage.Items)
	})

	t.Run("should get two pods from the list", func(t *testing.T) {
		// given
		var pods v1.PodList
		var expected v1.PodList
		pods.Items = []v1.Pod{
			*podWithExpectedImage,
			*podWithExpectedImageTerminating,
			*podWithExpectedImagePending,
			*podWithDifferentImageSuffix,
			*podWithDifferentImageSuffixTerminating,
			*podWithDifferentImageSuffixPending,
			*podWithDifferentImagePrefix,
			*podWithSoloImagePrefix,
		}
		expected.Items = []v1.Pod{
			*podWithDifferentImageSuffix,
			*podWithDifferentImagePrefix,
			*podWithSoloImagePrefix,
		}
		gatherer := DefaultGatherer{}

		// when
		podsWithDifferentImage := gatherer.GetPodsWithDifferentImage(pods, image)

		// then
		require.Equal(t, podsWithDifferentImage.Items, expected.Items)
		require.NotEmpty(t, podsWithDifferentImage.Items)
	})
}

func Test_Gatherer_GetPodsWithoutSidecar_sidecarInjectionEnabledByDefault(t *testing.T) {
	retryOpts := getTestingRetryOptions()

	podWithoutSidecarEnabledNS := fixPodWithoutSidecar("application", "enabled", "Running", map[string]string{}, map[string]string{})
	podWithoutSidecarNoLabeledNS := fixPodWithoutSidecar("application", "nolabel", "Running", map[string]string{}, map[string]string{})
	podWithoutSidecarDisabledNS := fixPodWithoutSidecar("application", "disabled", "Running", map[string]string{}, map[string]string{})
	podWithoutSidecarTerminating := fixPodWithoutSidecar("application2", "enabled", "Terminating", map[string]string{}, map[string]string{})
	podWithIstioSidecarEnabledNS := fixPodWithSidecar("application2", "enabled", "Running", map[string]string{}, map[string]string{})
	podWithIstioSidecarDisabledNS := fixPodWithSidecar("application2", "disabled", "Running", map[string]string{}, map[string]string{})
	truePodWithoutSidecar := fixPodWithoutSidecar("application", "enabled", "Running", map[string]string{"sidecar.istio.io/inject": "true"}, map[string]string{})
	truePodWithSidecar := fixPodWithSidecar("application", "enabled", "Running", map[string]string{"sidecar.istio.io/inject": "true"}, map[string]string{})
	truePodWithoutSidecarDisabledNS := fixPodWithoutSidecar("application", "disabled", "Running", map[string]string{"sidecar.istio.io/inject": "true"}, map[string]string{})
	falsePodWithoutSidecar := fixPodWithoutSidecar("application", "enabled", "Running", map[string]string{"sidecar.istio.io/inject": "false"}, map[string]string{})

	enabledNS := fixNamespaceWith("enabled", map[string]string{"istio-injection": "enabled"})
	disabledNS := fixNamespaceWith("disabled", map[string]string{"istio-injection": "disabled"})
	noLabelNS := fixNamespaceWith("nolabel", map[string]string{"testns": "true"})
	sidecarInjectionEnabledByDefault := true

	hostNetworkPod := fixPodWithoutSidecar("application", "nolabel", "Running", map[string]string{}, map[string]string{})
	hostNetworkPod.Spec.HostNetwork = true
	hostNetworkPodLabeled := fixPodWithoutSidecar("application1", "nolabel", "Running", map[string]string{"sidecar.istio.io/inject": "true"}, map[string]string{})
	hostNetworkPodLabeled.Spec.HostNetwork = true

	t.Run("should get pod with proper namespace label", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(podWithoutSidecarEnabledNS, podWithoutSidecarTerminating, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(podsWithoutSidecar.Items))
		require.NotEmpty(t, podsWithoutSidecar.Items)
	})
	t.Run("should not get pod with Istio sidecar", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(podWithIstioSidecarEnabledNS, noLabelNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsWithoutSidecar.Items))
	})
	t.Run("should get pod without Istio sidecar in namespace labeled istio-injection=enabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(podWithoutSidecarEnabledNS, podWithIstioSidecarEnabledNS, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(podsWithoutSidecar.Items))
	})
	t.Run("should not get pod without Istio sidecar in namespace labeled istio-injection=disabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(podWithoutSidecarDisabledNS, podWithIstioSidecarDisabledNS, disabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsWithoutSidecar.Items))
	})
	t.Run("should not get pod with Istio sidecar and annotated sidecar.istio.io/inject=true with in namespace labeled istio-injection=enabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(truePodWithSidecar, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsWithoutSidecar.Items))
	})
	t.Run("should get pod without Istio sidecar and annotated sidecar.istio.io/inject=true with in namespace labeled istio-injection=enabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(truePodWithoutSidecar, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(podsWithoutSidecar.Items))
	})
	t.Run("should not get pod without Istio sidecar and annotated sidecar.istio.io/inject=true with in namespace labeled istio-injection=disabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(truePodWithoutSidecarDisabledNS, disabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsWithoutSidecar.Items))
	})
	t.Run("should not get pod without Istio sidecar and annotated sidecar.istio.io/inject=false with in namespace labeled istio-injection=enabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(falsePodWithoutSidecar, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsWithoutSidecar.Items))
	})
	t.Run("should get pod without Istio sidecar when not annotated in namespace without label", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(podWithoutSidecarNoLabeledNS, noLabelNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(podsWithoutSidecar.Items))
	})
	t.Run("should not get pod in HostNetwork", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(hostNetworkPod, hostNetworkPodLabeled, noLabelNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsWithoutSidecar.Items))
	})
}

func Test_Gatherer_GetPodsWithoutSidecar_sidecarInjectionDisabledByDefault(t *testing.T) {
	retryOpts := getTestingRetryOptions()

	podWithoutSidecarEnabledNS := fixPodWithoutSidecar("application", "enabled", "Running", map[string]string{}, map[string]string{})
	podWithoutSidecarNoLabeledNS := fixPodWithoutSidecar("application", "nolabel", "Running", map[string]string{}, map[string]string{})
	podWithoutSidecarDisabledNS := fixPodWithoutSidecar("application", "disabled", "Running", map[string]string{}, map[string]string{})
	podWithoutSidecarTerminating := fixPodWithoutSidecar("application2", "enabled", "Terminating", map[string]string{}, map[string]string{})
	podWithIstioSidecarEnabledNS := fixPodWithSidecar("application2", "enabled", "Running", map[string]string{}, map[string]string{})
	podWithIstioSidecarEnabledNSTerminating := fixPodWithSidecar("application3", "enabled", "Terminating", map[string]string{}, map[string]string{})
	podWithIstioSidecarDisabledNS := fixPodWithSidecar("application2", "disabled", "Running", map[string]string{}, map[string]string{})
	truePodWithoutSidecar := fixPodWithoutSidecar("application", "enabled", "Running", map[string]string{"sidecar.istio.io/inject": "true"}, map[string]string{})
	truePodWithSidecar := fixPodWithSidecar("application", "enabled", "Running", map[string]string{"sidecar.istio.io/inject": "true"}, map[string]string{})
	truePodWithoutSidecarDisabledNS := fixPodWithoutSidecar("application", "disabled", "Running", map[string]string{"sidecar.istio.io/inject": "true"}, map[string]string{})
	falsePodWithoutSidecar := fixPodWithoutSidecar("application", "enabled", "Running", map[string]string{"sidecar.istio.io/inject": "false"}, map[string]string{})

	hostNetworkPod := fixPodWithoutSidecar("application", "nolabel", "Running", map[string]string{}, map[string]string{})
	hostNetworkPod.Spec.HostNetwork = true
	hostNetworkPodLabeled := fixPodWithoutSidecar("application1", "nolabel", "Running", map[string]string{"sidecar.istio.io/inject": "true"}, map[string]string{})
	hostNetworkPodLabeled.Spec.HostNetwork = true

	enabledNS := fixNamespaceWith("enabled", map[string]string{"istio-injection": "enabled"})
	disabledNS := fixNamespaceWith("disabled", map[string]string{"istio-injection": "disabled"})
	noLabelNS := fixNamespaceWith("nolabel", map[string]string{"testns": "true"})
	sidecarInjectionEnabledByDefault := false

	t.Run("should get pod with proper namespace label", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(podWithoutSidecarEnabledNS, podWithoutSidecarTerminating, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(podsWithoutSidecar.Items))
		require.NotEmpty(t, podsWithoutSidecar.Items)
	})
	t.Run("should not get pod with Istio sidecar", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(podWithIstioSidecarEnabledNS, noLabelNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsWithoutSidecar.Items))
	})
	t.Run("should get pod without Istio sidecar in namespace labeled istio-injection=enabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(podWithoutSidecarEnabledNS, podWithIstioSidecarEnabledNS, podWithIstioSidecarEnabledNSTerminating, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(podsWithoutSidecar.Items))
	})
	t.Run("should not get pod without Istio sidecar in namespace labeled istio-injection=disabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(podWithoutSidecarDisabledNS, podWithIstioSidecarDisabledNS, disabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsWithoutSidecar.Items))
	})
	t.Run("should not get pod with Istio sidecar and annotated sidecar.istio.io/inject=true with in namespace labeled istio-injection=enabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(truePodWithSidecar, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsWithoutSidecar.Items))
	})
	t.Run("should get pod without Istio sidecar and annotated sidecar.istio.io/inject=true with in namespace labeled istio-injection=enabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(truePodWithoutSidecar, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(podsWithoutSidecar.Items))
	})
	t.Run("should not get pod with Istio sidecar and annotated sidecar.istio.io/inject=true with in namespace labeled istio-injection=enabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(truePodWithSidecar, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsWithoutSidecar.Items))
	})
	t.Run("should not get pod without Istio sidecar and annotated sidecar.istio.io/inject=true with in namespace labeled istio-injection=disabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(truePodWithoutSidecarDisabledNS, disabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsWithoutSidecar.Items))
	})
	t.Run("should not get pod without Istio sidecar and annotated sidecar.istio.io/inject=false with in namespace labeled istio-injection=enabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(falsePodWithoutSidecar, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsWithoutSidecar.Items))
	})
	t.Run("should not get pod without Istio sidecar when not annotated in namespace without label", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(podWithoutSidecarNoLabeledNS, noLabelNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsWithoutSidecar.Items))
	})
	t.Run("should not get pod in HostNetwork", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(hostNetworkPodLabeled, hostNetworkPod, noLabelNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsWithoutSidecar.Items))
	})
}

func Test_Gatherer_GetPodsOutOfIstioMesh_sidecarInjectionEnabledByDefault(t *testing.T) {
	retryOpts := getTestingRetryOptions()

	podWithoutSidecarEnabledNS := fixPodWithoutSidecar("application", "enabled", "Running", map[string]string{}, map[string]string{})
	podWithoutSidecarNoLabeledNS := fixPodWithoutSidecar("application", "nolabel", "Running", map[string]string{}, map[string]string{})
	podWithoutSidecarDisabledNS := fixPodWithoutSidecar("application", "disabled", "Running", map[string]string{}, map[string]string{})
	podWithoutSidecarTerminating := fixPodWithoutSidecar("application2", "enabled", "Terminating", map[string]string{}, map[string]string{})
	podWithIstioSidecarEnabledNS := fixPodWithSidecar("application2", "enabled", "Running", map[string]string{}, map[string]string{})
	podWithIstioSidecarDisabledNS := fixPodWithSidecar("application2", "disabled", "Running", map[string]string{}, map[string]string{})
	truePodWithoutSidecar := fixPodWithoutSidecar("application", "enabled", "Running", map[string]string{"sidecar.istio.io/inject": "true"}, map[string]string{})
	truePodWithSidecar := fixPodWithSidecar("application", "enabled", "Running", map[string]string{"sidecar.istio.io/inject": "true"}, map[string]string{})
	truePodWithoutSidecarDisabledNS := fixPodWithoutSidecar("application", "disabled", "Running", map[string]string{"sidecar.istio.io/inject": "true"}, map[string]string{})
	falsePodWithoutSidecar := fixPodWithoutSidecar("application", "enabled", "Running", map[string]string{"sidecar.istio.io/inject": "false"}, map[string]string{})

	hostNetworkPod := fixPodWithoutSidecar("application", "nolabel", "Running", map[string]string{}, map[string]string{})
	hostNetworkPod.Spec.HostNetwork = true
	hostNetworkPodLabeled := fixPodWithoutSidecar("application1", "nolabel", "Running", map[string]string{"sidecar.istio.io/inject": "false"}, map[string]string{})
	hostNetworkPodLabeled.Spec.HostNetwork = true

	enabledNS := fixNamespaceWith("enabled", map[string]string{"istio-injection": "enabled"})
	disabledNS := fixNamespaceWith("disabled", map[string]string{"istio-injection": "disabled"})
	noLabelNS := fixNamespaceWith("nolabel", map[string]string{"testns": "true"})
	sidecarInjectionEnabledByDefault := true

	t.Run("should not get pod with proper namespace label", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(podWithoutSidecarEnabledNS, podWithoutSidecarTerminating, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsOutOfIstioMesh, err := gatherer.GetPodsOutOfIstioMesh(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsOutOfIstioMesh.Items))
	})
	t.Run("should not get not labeled pod", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(podWithIstioSidecarEnabledNS, noLabelNS)
		gatherer := DefaultGatherer{}

		// when
		podsOutOfIstioMesh, err := gatherer.GetPodsOutOfIstioMesh(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsOutOfIstioMesh.Items))
	})
	t.Run("should not get pod without Istio sidecar in namespace labeled istio-injection=enabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(podWithoutSidecarEnabledNS, podWithIstioSidecarEnabledNS, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsOutOfIstioMesh, err := gatherer.GetPodsOutOfIstioMesh(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsOutOfIstioMesh.Items))
	})
	t.Run("should get all pods in namespace Istio sidecar in namespace labeled istio-injection=disabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(podWithoutSidecarDisabledNS, podWithIstioSidecarDisabledNS, disabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsOutOfIstioMesh, err := gatherer.GetPodsOutOfIstioMesh(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 2, len(podsOutOfIstioMesh.Items))
	})
	t.Run("should not get pod with Istio sidecar and annotated sidecar.istio.io/inject=true with in namespace labeled istio-injection=enabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(truePodWithSidecar, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsOutOfIstioMesh, err := gatherer.GetPodsOutOfIstioMesh(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsOutOfIstioMesh.Items))
	})
	t.Run("should not get pod without Istio sidecar and annotated sidecar.istio.io/inject=true with in namespace labeled istio-injection=enabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(truePodWithoutSidecar, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsOutOfIstioMesh, err := gatherer.GetPodsOutOfIstioMesh(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsOutOfIstioMesh.Items))
	})
	t.Run("should get pod without Istio sidecar and annotated sidecar.istio.io/inject=true with in namespace labeled istio-injection=disabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(truePodWithoutSidecarDisabledNS, disabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsOutOfIstioMesh, err := gatherer.GetPodsOutOfIstioMesh(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(podsOutOfIstioMesh.Items))
	})
	t.Run("should get pod without Istio sidecar and annotated sidecar.istio.io/inject=false with in namespace labeled istio-injection=enabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(falsePodWithoutSidecar, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsOutOfIstioMesh, err := gatherer.GetPodsOutOfIstioMesh(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(podsOutOfIstioMesh.Items))
	})
	t.Run("should not get pod without Istio sidecar when not annotated in namespace without label", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(podWithoutSidecarNoLabeledNS, noLabelNS)
		gatherer := DefaultGatherer{}

		// when
		podsOutOfIstioMesh, err := gatherer.GetPodsOutOfIstioMesh(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsOutOfIstioMesh.Items))
	})
	t.Run("should not get pod without Istio sidecar when not annotated in namespace without label", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(podWithoutSidecarNoLabeledNS, noLabelNS)
		gatherer := DefaultGatherer{}

		// when
		podsOutOfIstioMesh, err := gatherer.GetPodsOutOfIstioMesh(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsOutOfIstioMesh.Items))
	})
	t.Run("should get all pods in HostNetwork", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(hostNetworkPod, hostNetworkPodLabeled, noLabelNS)
		gatherer := DefaultGatherer{}

		// when
		podsOutOfIstioMesh, err := gatherer.GetPodsOutOfIstioMesh(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 2, len(podsOutOfIstioMesh.Items))
	})
}

func Test_Gatherer_GetPodsOutOfIstioMesh_sidecarInjectionDisabledByDefault(t *testing.T) {
	retryOpts := getTestingRetryOptions()

	podWithoutSidecarEnabledNS := fixPodWithoutSidecar("application", "enabled", "Running", map[string]string{}, map[string]string{})
	podWithoutSidecarNoLabeledNS := fixPodWithoutSidecar("application", "nolabel", "Running", map[string]string{}, map[string]string{})
	podWithoutSidecarDisabledNS := fixPodWithoutSidecar("application", "disabled", "Running", map[string]string{}, map[string]string{})
	podWithoutSidecarTerminating := fixPodWithoutSidecar("application2", "enabled", "Terminating", map[string]string{}, map[string]string{})
	podWithIstioSidecarEnabledNS := fixPodWithSidecar("application2", "enabled", "Running", map[string]string{}, map[string]string{})
	podWithIstioSidecarEnabledNSTerminating := fixPodWithSidecar("application3", "enabled", "Terminating", map[string]string{}, map[string]string{})
	podWithIstioSidecarDisabledNS := fixPodWithSidecar("application2", "disabled", "Running", map[string]string{}, map[string]string{})
	truePodWithoutSidecar := fixPodWithoutSidecar("application", "enabled", "Running", map[string]string{"sidecar.istio.io/inject": "true"}, map[string]string{})
	truePodWithSidecar := fixPodWithSidecar("application", "enabled", "Running", map[string]string{"sidecar.istio.io/inject": "true"}, map[string]string{})
	labeledPodWithoutSidecarDisabledNS := fixPodWithoutSidecar("application", "disabled", "Running", map[string]string{}, map[string]string{"sidecar.istio.io/inject": "true"})
	truePodWithoutSidecarDisabledNS := fixPodWithoutSidecar("application", "disabled", "Running", map[string]string{"sidecar.istio.io/inject": "true"}, map[string]string{})
	falsePodWithoutSidecar := fixPodWithoutSidecar("application", "enabled", "Running", map[string]string{"sidecar.istio.io/inject": "false"}, map[string]string{})

	hostNetworkPod := fixPodWithoutSidecar("application", "nolabel", "Running", map[string]string{}, map[string]string{})
	hostNetworkPod.Spec.HostNetwork = true
	hostNetworkPodLabeled := fixPodWithoutSidecar("application1", "nolabel", "Running", map[string]string{"sidecar.istio.io/inject": "false"}, map[string]string{})
	hostNetworkPodLabeled.Spec.HostNetwork = true

	enabledNS := fixNamespaceWith("enabled", map[string]string{"istio-injection": "enabled"})
	disabledNS := fixNamespaceWith("disabled", map[string]string{"istio-injection": "disabled"})
	noLabelNS := fixNamespaceWith("nolabel", map[string]string{"testns": "true"})
	sidecarInjectionEnabledByDefault := false

	t.Run("should not get pod with proper namespace label", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(podWithoutSidecarEnabledNS, podWithoutSidecarTerminating, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsOutOfIstioMesh, err := gatherer.GetPodsOutOfIstioMesh(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Empty(t, podsOutOfIstioMesh.Items)
	})
	t.Run("should not get pod with Istio sidecar", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(podWithIstioSidecarEnabledNS, noLabelNS)
		gatherer := DefaultGatherer{}

		// when
		podsOutOfIstioMesh, err := gatherer.GetPodsOutOfIstioMesh(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsOutOfIstioMesh.Items))
	})
	t.Run("should not get pod without Istio sidecar in namespace labeled istio-injection=enabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(podWithoutSidecarEnabledNS, podWithIstioSidecarEnabledNS, podWithIstioSidecarEnabledNSTerminating, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsOutOfIstioMesh, err := gatherer.GetPodsOutOfIstioMesh(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Empty(t, podsOutOfIstioMesh.Items)
	})
	t.Run("should get pods in namespace labeled istio-injection=disabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(podWithoutSidecarDisabledNS, podWithIstioSidecarDisabledNS, disabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsOutOfIstioMesh, err := gatherer.GetPodsOutOfIstioMesh(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 2, len(podsOutOfIstioMesh.Items))
	})
	t.Run("should not get pod with Istio sidecar and annotated sidecar.istio.io/inject=true with in namespace labeled istio-injection=enabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(truePodWithSidecar, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsOutOfIstioMesh, err := gatherer.GetPodsOutOfIstioMesh(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsOutOfIstioMesh.Items))
	})
	t.Run("should not get pod without Istio sidecar and annotated sidecar.istio.io/inject=true with in namespace labeled istio-injection=enabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(truePodWithoutSidecar, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsOutOfIstioMesh, err := gatherer.GetPodsOutOfIstioMesh(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsOutOfIstioMesh.Items))
	})
	t.Run("should not get pod with Istio sidecar and annotated sidecar.istio.io/inject=true with in namespace labeled istio-injection=enabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(truePodWithSidecar, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsOutOfIstioMesh, err := gatherer.GetPodsOutOfIstioMesh(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsOutOfIstioMesh.Items))
	})
	t.Run("should get pod without Istio sidecar and annotated sidecar.istio.io/inject=true with in namespace labeled istio-injection=disabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(truePodWithoutSidecarDisabledNS, disabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsOutOfIstioMesh, err := gatherer.GetPodsOutOfIstioMesh(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(podsOutOfIstioMesh.Items))
	})
	t.Run("should get pod without Istio sidecar and labeled sidecar.istio.io/inject=true with in namespace labeled istio-injection=disabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(labeledPodWithoutSidecarDisabledNS, disabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsOutOfIstioMesh, err := gatherer.GetPodsOutOfIstioMesh(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(podsOutOfIstioMesh.Items))
	})
	t.Run("should get pod without Istio sidecar and annotated sidecar.istio.io/inject=false with in namespace labeled istio-injection=enabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(falsePodWithoutSidecar, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsOutOfIstioMesh, err := gatherer.GetPodsOutOfIstioMesh(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(podsOutOfIstioMesh.Items))
	})
	t.Run("should get pod without Istio sidecar when not annotated in namespace without label", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(podWithoutSidecarNoLabeledNS, noLabelNS)
		gatherer := DefaultGatherer{}

		// when
		podsOutOfIstioMesh, err := gatherer.GetPodsOutOfIstioMesh(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(podsOutOfIstioMesh.Items))
	})
	t.Run("should get all pods in HostNetwork", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(hostNetworkPod, hostNetworkPodLabeled, noLabelNS)
		gatherer := DefaultGatherer{}

		// when
		podsOutOfIstioMesh, err := gatherer.GetPodsOutOfIstioMesh(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 2, len(podsOutOfIstioMesh.Items))
	})
}

func fixPodWith(name, namespace, image, phase string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "ReplicaSet"},
			},
			Annotations: map[string]string{"sidecar.istio.io/status": fmt.Sprintf(`{"containers":["%s"]}`, name+"-containertwo")},
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		Status: v1.PodStatus{
			Phase: v1.PodPhase(phase),
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  name + "-container",
					Image: "wrongimage:6.9",
				},
				{
					Name:  name + "-containertwo",
					Image: image,
				},
			},
		},
	}
}

func fixPodWithSidecar(name, namespace, phase string, annotations map[string]string, labels map[string]string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "ReplicaSet"},
			},
			Labels:      labels,
			Annotations: annotations,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		Status: v1.PodStatus{
			Phase: v1.PodPhase(phase),
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  name + "-container",
					Image: "image:6.9",
				},
				{
					Name:  "istio-proxy",
					Image: "istio-proxy",
				},
			},
		},
	}
}

func fixPodWithoutSidecar(name, namespace, phase string, annotations map[string]string, labels map[string]string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "ReplicaSet"},
			},
			Labels:      labels,
			Annotations: annotations,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		Status: v1.PodStatus{
			Phase: v1.PodPhase(phase),
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  name + "-container",
					Image: "image:6.9",
				},
			},
		},
	}
}

func fixNamespaceWith(name string, labels map[string]string) *v1.Namespace {
	return &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}

func TestRemoveAnnotatedPods(t *testing.T) {

	t.Run("should not filter not annotated pods", func(t *testing.T) {
		in := v1.PodList{Items: []v1.Pod{{}, {}}}
		got := RemoveAnnotatedPods(in, "foo")
		require.Equal(t, len(in.Items), len(got.Items))
	})

	t.Run("should not filter pods that don't match annotation", func(t *testing.T) {
		in := v1.PodList{Items: []v1.Pod{{
			ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"foo": "bar"}},
		}, {}}}
		got := RemoveAnnotatedPods(in, "baz")
		require.Equal(t, len(in.Items), len(got.Items))
	})

	t.Run("should filter annotated pods", func(t *testing.T) {
		in := v1.PodList{Items: []v1.Pod{{
			ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"foo": "bar"}},
		}, {}}}
		got := RemoveAnnotatedPods(in, "foo")
		require.Equal(t, in.Items[1:], got.Items)
	})

	t.Run("should filter all pods if all are annotated", func(t *testing.T) {
		in := v1.PodList{Items: []v1.Pod{{
			ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"foo": "bar"}},
		}, {
			ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"foo": "bar"}},
		}}}
		got := RemoveAnnotatedPods(in, "foo")
		require.Equal(t, 0, len(got.Items))
	})

}

func getTestingRetryOptions() []retry.Option {
	return []retry.Option{
		retry.Delay(0),
		retry.Attempts(1),
		retry.DelayType(retry.FixedDelay),
	}
}
