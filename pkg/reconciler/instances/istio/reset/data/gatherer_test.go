package data

import (
	"fmt"
	"testing"

	"github.com/avast/retry-go"
	"github.com/kyma-incubator/reconciler/pkg/logger"
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
		require.Empty(t, pods.Items)
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

func Test_GetIstioCPPods(t *testing.T) {
	firstPod := fixPodWith("istiod", "istio-system", "istio/pilot:1.1.0", "Running")
	secondPod := fixPodWith("istio-ingressgateway", "istio-system", "istio/proxyv2:1.1.0", "Running")
	thirdPod := fixPodWith("application", "kyma", "istio/proxyv2:1.10.1", "Running")
	retryOpts := getTestingRetryOptions()

	t.Run("should not get any pods from the cluster when there are no pods in istio-system namespace", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(thirdPod)
		gatherer := DefaultGatherer{}

		// when
		pods, err := gatherer.GetIstioCPPods(kubeClient, retryOpts)

		// then
		require.NoError(t, err)
		require.Empty(t, pods.Items)
	})

	t.Run("should get all pods from the cluster in istio-system namespace", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(firstPod, secondPod, thirdPod)
		gatherer := DefaultGatherer{}

		// when
		pods, err := gatherer.GetIstioCPPods(kubeClient, retryOpts)

		// then
		require.NoError(t, err)
		require.Len(t, pods.Items, 2)
	})
}

func Test_GetInstalledIstioVersion(t *testing.T) {
	log := logger.NewLogger(false)
	istiodPod := fixPodWithContainerName("istiod", "istio-system", "istio/pilot:1.1.0", "discovery", false)
	istiogwPod := fixPodWithContainerName("istio-ingressgateway", "istio-system", "istio/proxyv2:1.1.0", "istio-proxy", false)
	istiogwPodTerm := fixPodWithContainerName("istio-ingressgateway-old", "istio-system", "istio/proxyv2:1.0.0", "istio-proxy", true)
	istiocniPod := fixPodWithContainerName("istio-cni-node", "istio-system", "istio/install-cni:1.1.0", "install-cni", false)
	appPod := fixPodWithContainerName("httpbin", "kyma", "istio/proxyv2:1.0.1", "istio-proxy", false)
	retryOpts := getTestingRetryOptions()

	t.Run("should get Istio installed version based on pods in istio-system namespace", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(istiodPod, istiogwPod, istiocniPod, appPod)
		gatherer := DefaultGatherer{}

		// when
		version, err := gatherer.GetInstalledIstioVersion(kubeClient, retryOpts, log)

		// then
		require.NoError(t, err)
		require.Equal(t, version, "1.1.0")
	})

	t.Run("should get Istio installed version based on pods in istio-system namespace when some pods are terminating", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(istiodPod, istiogwPodTerm, istiogwPod, istiocniPod, appPod)
		gatherer := DefaultGatherer{}

		// when
		version, err := gatherer.GetInstalledIstioVersion(kubeClient, retryOpts, log)

		// then
		require.NoError(t, err)
		require.Equal(t, version, "1.1.0")
	})

	t.Run("should get Istio installed version when there is a pod with image prerelease version", func(t *testing.T) {
		// given
		istiocniPod := fixPodWithContainerName("istio-cni-node", "istio-system", "istio/install-cni:1.1.0-distorelss", "install-cni", false)
		kubeClient := fake.NewSimpleClientset(istiodPod, istiogwPod, istiocniPod, appPod)
		gatherer := DefaultGatherer{}

		// when
		version, err := gatherer.GetInstalledIstioVersion(kubeClient, retryOpts, log)

		// then
		require.NoError(t, err)
		require.Equal(t, version, "1.1.0")
	})

	t.Run("should return error when there are no pods in istio-namespace", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset()
		gatherer := DefaultGatherer{}

		// when
		version, err := gatherer.GetInstalledIstioVersion(kubeClient, retryOpts, log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Unable to obtain installed Istio image version")
		require.Equal(t, version, "")
	})

	t.Run("should return error when there is an inconsistent version state in istio-system namespace", func(t *testing.T) {
		// given
		istiocniPod := fixPodWithContainerName("istio-cni-node", "istio-system", "istio/install-cni:1.0.0", "install-cni", false)
		kubeClient := fake.NewSimpleClientset(istiodPod, istiogwPod, istiocniPod, appPod)
		gatherer := DefaultGatherer{}

		// when
		version, err := gatherer.GetInstalledIstioVersion(kubeClient, retryOpts, log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Image version of pod istio-ingressgateway: 1.1.0 do not match version: 1.0.0")
		require.Equal(t, version, "")
	})

	t.Run("should return error when there is a pod with wrong image", func(t *testing.T) {
		// given
		istiocniPod := fixPodWithContainerName("istio-cni-node", "istio-system", "wrong", "install-cni", false)
		kubeClient := fake.NewSimpleClientset(istiodPod, istiogwPod, istiocniPod, appPod)
		gatherer := DefaultGatherer{}

		// when
		version, err := gatherer.GetInstalledIstioVersion(kubeClient, retryOpts, log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid istioctl version format: empty input")
		require.Equal(t, version, "")
	})

	t.Run("should return error when there is a pod with latest versioned image", func(t *testing.T) {
		// given
		istiocniPod := fixPodWithContainerName("istio-cni-node", "istio-system", "istio/install-cni:latest", "install-cni", false)
		kubeClient := fake.NewSimpleClientset(istiodPod, istiogwPod, istiocniPod, appPod)
		gatherer := DefaultGatherer{}

		// when
		version, err := gatherer.GetInstalledIstioVersion(kubeClient, retryOpts, log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "is not in dotted-tri format")
		require.Equal(t, version, "")
	})
}

func Test_Gatherer_GetPodsForCNIChange(t *testing.T) {
	retryOpts := getTestingRetryOptions()
	enabledNS := fixNamespaceWith("enabled", map[string]string{"istio-injection": "enabled"})
	disabledNS := fixNamespaceWith("disabled", map[string]string{"istio-injection": "disabled"})
	t.Run("should not get any pod without istio-init container when CNI is enabled", func(t *testing.T) {
		// given
		cniEnabled := true
		firstPod := fixPodWithoutInitContainer("application1", "enabled", "Running", map[string]string{}, map[string]string{})
		secondPod := fixPodWithoutInitContainer("application2", "enabled", "Terminating", map[string]string{}, map[string]string{})

		kubeClient := fake.NewSimpleClientset(firstPod, secondPod, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		pods, err := gatherer.GetPodsForCNIChange(kubeClient, retryOpts, cniEnabled)

		// then
		require.NoError(t, err)
		require.Empty(t, pods.Items)
	})
	t.Run("should get 2 pods with istio-init when they are in Running state when CNI is enabled", func(t *testing.T) {
		// given
		cniEnabled := true
		firstPod := fixPodWithSidecar("application2", "enabled", "Running", map[string]string{}, map[string]string{})
		secondPod := fixPodWithSidecar("application1", "enabled", "Running", map[string]string{}, map[string]string{})
		kubeClient := fake.NewSimpleClientset(firstPod, secondPod, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		pods, err := gatherer.GetPodsForCNIChange(kubeClient, retryOpts, cniEnabled)

		// then
		require.NoError(t, err)
		require.Len(t, pods.Items, 2)
	})
	t.Run("should not get pod with istio-init in Terminating state", func(t *testing.T) {
		// given
		cniEnabled := true
		firstPod := fixPodWithSidecar("application2", "enabled", "Running", map[string]string{}, map[string]string{})
		secondPod := fixPodWithSidecar("application1", "enabled", "Terminating", map[string]string{}, map[string]string{})
		kubeClient := fake.NewSimpleClientset(firstPod, secondPod, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		pods, err := gatherer.GetPodsForCNIChange(kubeClient, retryOpts, cniEnabled)

		// then
		require.NoError(t, err)
		require.Len(t, pods.Items, 1)
	})
	t.Run("should not get any pod with istio-validation container when CNI is enabled", func(t *testing.T) {
		// given
		cniEnabled := true
		firstPod := fixPodWithoutInitContainer("application2", "enabled", "Running", map[string]string{}, map[string]string{})
		secondPod := fixPodWithoutInitContainer("application1", "enabled", "Running", map[string]string{}, map[string]string{})
		kubeClient := fake.NewSimpleClientset(firstPod, secondPod, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		pods, err := gatherer.GetPodsForCNIChange(kubeClient, retryOpts, cniEnabled)

		// then
		require.NoError(t, err)
		require.Len(t, pods.Items, 0)
	})
	t.Run("should get 2 pods with istio-validation container when CNI is disabled", func(t *testing.T) {
		// given
		cniEnabled := false
		firstPod := fixPodWithoutInitContainer("application2", "enabled", "Running", map[string]string{}, map[string]string{})
		secondPod := fixPodWithoutInitContainer("application1", "enabled", "Running", map[string]string{}, map[string]string{})
		kubeClient := fake.NewSimpleClientset(firstPod, secondPod, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		pods, err := gatherer.GetPodsForCNIChange(kubeClient, retryOpts, cniEnabled)

		// then
		require.NoError(t, err)
		require.Len(t, pods.Items, 2)
	})
	t.Run("should not get any pod with istio-validation container in disabled namespace when CNI is disabled", func(t *testing.T) {
		// given
		cniEnabled := true
		pod := fixPodWithoutInitContainer("application1", "disabled", "Running", map[string]string{}, map[string]string{})
		kubeClient := fake.NewSimpleClientset(pod, disabledNS)
		gatherer := DefaultGatherer{}

		// when
		pods, err := gatherer.GetPodsForCNIChange(kubeClient, retryOpts, cniEnabled)

		// then
		require.NoError(t, err)
		require.Empty(t, pods.Items)
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
	annotatedTrueLabeledFalsePodWithoutSidecarEnabledNS := fixPodWithoutSidecar("application", "enabled", "Running", map[string]string{"sidecar.istio.io/inject": "true"}, map[string]string{"sidecar.istio.io/inject": "false"})
	annotatedFalseLabeledTruePodWithoutSidecarEnabledNS := fixPodWithoutSidecar("application", "enabled", "Running", map[string]string{"sidecar.istio.io/inject": "false"}, map[string]string{"sidecar.istio.io/inject": "true"})
	annotatedTruePodWithoutSidecar := fixPodWithoutSidecar("application", "enabled", "Running", map[string]string{"sidecar.istio.io/inject": "true"}, map[string]string{})
	annotatedTruePodWithSidecar := fixPodWithSidecar("application", "enabled", "Running", map[string]string{"sidecar.istio.io/inject": "true"}, map[string]string{})
	annotatedTruePodWithoutSidecarDisabledNS := fixPodWithoutSidecar("application", "disabled", "Running", map[string]string{"sidecar.istio.io/inject": "true"}, map[string]string{})
	annotatedFalsePodWithoutSidecar := fixPodWithoutSidecar("application", "enabled", "Running", map[string]string{"sidecar.istio.io/inject": "false"}, map[string]string{})
	annotatedTruePodWithoutSidecarNoLabelNS := fixPodWithoutSidecar("application", "nolabel", "Running", map[string]string{"sidecar.istio.io/inject": "true"}, map[string]string{})
	labeledTruePodWithoutSidecarNoLabelNS := fixPodWithoutSidecar("application", "nolabel", "Running", map[string]string{}, map[string]string{"sidecar.istio.io/inject": "true"})

	enabledNS := fixNamespaceWith("enabled", map[string]string{"istio-injection": "enabled"})
	disabledNS := fixNamespaceWith("disabled", map[string]string{"istio-injection": "disabled"})
	noLabelNS := fixNamespaceWith("nolabel", map[string]string{"testns": "true"})
	sidecarInjectionEnabledByDefault := true

	hostNetworkPod := fixPodWithoutSidecar("application", "nolabel", "Running", map[string]string{}, map[string]string{})
	hostNetworkPod.Spec.HostNetwork = true
	hostNetworkPodAnnotated := fixPodWithoutSidecar("application1", "nolabel", "Running", map[string]string{"sidecar.istio.io/inject": "true"}, map[string]string{})
	hostNetworkPodAnnotated.Spec.HostNetwork = true

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
		kubeClient := fake.NewSimpleClientset(annotatedTruePodWithSidecar, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsWithoutSidecar.Items))
	})
	t.Run("should get pod without Istio sidecar and annotated sidecar.istio.io/inject=true with in namespace labeled istio-injection=enabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(annotatedTruePodWithoutSidecar, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(podsWithoutSidecar.Items))
	})
	t.Run("should not get pod without Istio sidecar and annotated sidecar.istio.io/inject=true with in namespace labeled istio-injection=disabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(annotatedTruePodWithoutSidecarDisabledNS, disabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsWithoutSidecar.Items))
	})
	t.Run("should not get pod without Istio sidecar and annotated sidecar.istio.io/inject=false with in namespace labeled istio-injection=enabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(annotatedFalsePodWithoutSidecar, enabledNS)
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
		kubeClient := fake.NewSimpleClientset(hostNetworkPod, hostNetworkPodAnnotated, noLabelNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsWithoutSidecar.Items))
	})
	t.Run("should get pod without Istio sidecar and annotated sidecar.istio.io/inject=true in namespace without label", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(annotatedTruePodWithoutSidecarNoLabelNS, noLabelNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(podsWithoutSidecar.Items))
	})
	t.Run("should get pod without Istio sidecar and labeled sidecar.istio.io/inject=true in namespace without label", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(labeledTruePodWithoutSidecarNoLabelNS, noLabelNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(podsWithoutSidecar.Items))
	})
	t.Run("should not get pod without Istio sidecar and annotated sidecar.istio.io/inject=true and labeled sidecar.istio.io/inject=false in namespace labeled istio-injection=enabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(annotatedTrueLabeledFalsePodWithoutSidecarEnabledNS, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsWithoutSidecar.Items))
	})
	t.Run("should get pod without Istio sidecar and annotated sidecar.istio.io/inject=false and labeled sidecar.istio.io/inject=true in namespace labeled istio-injection=enabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(annotatedFalseLabeledTruePodWithoutSidecarEnabledNS, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(podsWithoutSidecar.Items))
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
	annotatedTrueLabeledFalsePodWithoutSidecarEnabledNS := fixPodWithoutSidecar("application", "enabled", "Running", map[string]string{"sidecar.istio.io/inject": "true"}, map[string]string{"sidecar.istio.io/inject": "false"})
	annotatedFalseLabeledTruePodWithoutSidecarEnabledNS := fixPodWithoutSidecar("application", "enabled", "Running", map[string]string{"sidecar.istio.io/inject": "false"}, map[string]string{"sidecar.istio.io/inject": "true"})
	annotatedTruePodWithoutSidecar := fixPodWithoutSidecar("application", "enabled", "Running", map[string]string{"sidecar.istio.io/inject": "true"}, map[string]string{})
	annotatedTruePodWithSidecar := fixPodWithSidecar("application", "enabled", "Running", map[string]string{"sidecar.istio.io/inject": "true"}, map[string]string{})
	annotatedTruePodWithoutSidecarDisabledNS := fixPodWithoutSidecar("application", "disabled", "Running", map[string]string{"sidecar.istio.io/inject": "true"}, map[string]string{})
	annotatedFalsePodWithoutSidecar := fixPodWithoutSidecar("application", "enabled", "Running", map[string]string{"sidecar.istio.io/inject": "false"}, map[string]string{})
	annotatedTruePodWithoutSidecarNoLabelNS := fixPodWithoutSidecar("application", "nolabel", "Running", map[string]string{"sidecar.istio.io/inject": "true"}, map[string]string{})
	labeledTruePodWithoutSidecarNoLabelNS := fixPodWithoutSidecar("application", "nolabel", "Running", map[string]string{}, map[string]string{"sidecar.istio.io/inject": "true"})

	hostNetworkPod := fixPodWithoutSidecar("application", "nolabel", "Running", map[string]string{}, map[string]string{})
	hostNetworkPod.Spec.HostNetwork = true
	hostNetworkPodAnnotated := fixPodWithoutSidecar("application1", "nolabel", "Running", map[string]string{"sidecar.istio.io/inject": "true"}, map[string]string{})
	hostNetworkPodAnnotated.Spec.HostNetwork = true

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
		kubeClient := fake.NewSimpleClientset(annotatedTruePodWithSidecar, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsWithoutSidecar.Items))
	})
	t.Run("should get pod without Istio sidecar and annotated sidecar.istio.io/inject=true with in namespace labeled istio-injection=enabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(annotatedTruePodWithoutSidecar, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(podsWithoutSidecar.Items))
	})
	t.Run("should not get pod with Istio sidecar and annotated sidecar.istio.io/inject=true with in namespace labeled istio-injection=enabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(annotatedTruePodWithSidecar, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsWithoutSidecar.Items))
	})
	t.Run("should not get pod without Istio sidecar and annotated sidecar.istio.io/inject=true with in namespace labeled istio-injection=disabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(annotatedTruePodWithoutSidecarDisabledNS, disabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsWithoutSidecar.Items))
	})
	t.Run("should not get pod without Istio sidecar and annotated sidecar.istio.io/inject=false with in namespace labeled istio-injection=enabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(annotatedFalsePodWithoutSidecar, enabledNS)
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
		kubeClient := fake.NewSimpleClientset(hostNetworkPodAnnotated, hostNetworkPod, noLabelNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsWithoutSidecar.Items))
	})
	t.Run("should not get pod without Istio sidecar and annotated sidecar.istio.io/inject=true in namespace without label", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(annotatedTruePodWithoutSidecarNoLabelNS, noLabelNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsWithoutSidecar.Items))
	})
	t.Run("should get pod without Istio sidecar and labeled sidecar.istio.io/inject=true in namespace without label", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(labeledTruePodWithoutSidecarNoLabelNS, noLabelNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(podsWithoutSidecar.Items))
	})
	t.Run("should not get pod without Istio sidecar and annotated sidecar.istio.io/inject=true and labeled sidecar.istio.io/inject=false in namespace labeled istio-injection=enabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(annotatedTrueLabeledFalsePodWithoutSidecarEnabledNS, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 0, len(podsWithoutSidecar.Items))
	})
	t.Run("should get pod without Istio sidecar and annotated sidecar.istio.io/inject=false and labeled sidecar.istio.io/inject=true in namespace labeled istio-injection=enabled", func(t *testing.T) {
		// given
		kubeClient := fake.NewSimpleClientset(annotatedFalseLabeledTruePodWithoutSidecarEnabledNS, enabledNS)
		gatherer := DefaultGatherer{}

		// when
		podsWithoutSidecar, err := gatherer.GetPodsWithoutSidecar(kubeClient, retryOpts, sidecarInjectionEnabledByDefault)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(podsWithoutSidecar.Items))
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

func fixPodWithContainerName(name, namespace, image, containerName string, terminating bool) *v1.Pod {
	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "ReplicaSet"},
			},
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		Status: v1.PodStatus{
			Phase: v1.PodPhase("Running"),
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  containerName,
					Image: image,
				},
				{
					Name:  containerName + "-2",
					Image: image,
				},
			},
		},
	}
	if terminating {
		timestamp := metav1.Now()
		pod.ObjectMeta.DeletionTimestamp = &timestamp
	}
	return &pod
}

func fixPodWithoutInitContainer(name, namespace, phase string, annotations map[string]string, labels map[string]string) *v1.Pod {
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
			InitContainers: []v1.Container{
				{
					Name:  "istio-validation",
					Image: "istio-validation",
				},
			},
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
			InitContainers: []v1.Container{
				{
					Name:  "istio-init",
					Image: "istio-init",
				},
			},
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
