package webhooks

import (
	"context"
	"errors"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	clientsetmocks "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/clientset/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"istio.io/istio/istioctl/pkg/tag"
	v1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	revLabelKey   = "istio.io/rev"
	defaultWhName = "istio-sidecar-injector"
	taggedWhName  = "istio-revision-tag-default"
)

var validSelector = &metav1.LabelSelector{
	MatchExpressions: []metav1.LabelSelectorRequirement{{
		Key:      "istio-injection",
		Operator: "DoesNotExist",
	}},
}

var deactivatedSelector = &metav1.LabelSelector{
	MatchLabels: deactivatedLabel,
}

func createMutatingWebhookWithSelector(whConfName string, labels map[string]string, selector *metav1.LabelSelector) *v1.MutatingWebhookConfiguration {
	return &v1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   whConfName,
			Labels: labels,
		},
		Webhooks: []v1.MutatingWebhook{
			{
				Name:              "object.sidecar-injector.istio.io",
				NamespaceSelector: selector,
				ObjectSelector:    selector,
			},
		},
	}
}

func Test_DeleteConflictedDefaultTag(t *testing.T) {
	kubeConfig := "kubeconfig"
	log := logger.NewLogger(false)
	ctx := context.Background()

	defer ctx.Done()

	t.Run("should return error when kubeclient could not be retrieved", func(t *testing.T) {
		// given
		provider := clientsetmocks.Provider{}
		provider.On("RetrieveFrom", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(nil, errors.New("Kubernetes client error"))

		// when
		err := DeleteConflictedDefaultTag(ctx, &provider, kubeConfig, log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Kubernetes client error")
	})
	t.Run("should not delete tagged webhook when old webhook is deactivated", func(t *testing.T) {
		// given
		defaultMwcObj := createMutatingWebhookWithSelector(defaultWhName, map[string]string{revLabelKey: tag.DefaultRevisionName}, deactivatedSelector)
		taggedMwcObj := createMutatingWebhookWithSelector(taggedWhName, map[string]string{tag.IstioTagLabel: tag.DefaultRevisionName, revLabelKey: tag.DefaultRevisionName}, validSelector)
		client := fake.NewSimpleClientset(defaultMwcObj, taggedMwcObj)
		provider := &clientsetmocks.Provider{}
		provider.On("RetrieveFrom", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(client, nil)
		// when
		err := DeleteConflictedDefaultTag(ctx, provider, kubeConfig, log)

		// then
		require.NoError(t, err)
		taggedMwc, err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, taggedWhName, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, taggedMwcObj, taggedMwc)
		defaultMwc, err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, defaultWhName, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, defaultMwcObj, defaultMwc)
	})
	t.Run("should delete conflicted tagged webhook if old one is not deactivated", func(t *testing.T) {
		// given
		defaultMwcObj := createMutatingWebhookWithSelector(defaultWhName, map[string]string{revLabelKey: tag.DefaultRevisionName}, validSelector)
		taggedMwcObj := createMutatingWebhookWithSelector(taggedWhName, map[string]string{tag.IstioTagLabel: tag.DefaultRevisionName, revLabelKey: tag.DefaultRevisionName}, validSelector)
		client := fake.NewSimpleClientset(defaultMwcObj, taggedMwcObj)
		provider := &clientsetmocks.Provider{}
		provider.On("RetrieveFrom", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(client, nil)
		// when
		err := DeleteConflictedDefaultTag(ctx, provider, kubeConfig, log)

		// then
		require.NoError(t, err)
		taggedMwc, err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, taggedWhName, metav1.GetOptions{})
		require.ErrorContains(t, err, "not found")
		require.Nil(t, taggedMwc)
		defaultMwc, err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, defaultWhName, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, defaultMwcObj, defaultMwc)
	})
	t.Run("should not delete tagged webhook if there is no old default webhook", func(t *testing.T) {
		// given
		taggedMwcObj := createMutatingWebhookWithSelector(taggedWhName, map[string]string{tag.IstioTagLabel: tag.DefaultRevisionName, revLabelKey: tag.DefaultRevisionName}, validSelector)
		client := fake.NewSimpleClientset(taggedMwcObj)
		provider := &clientsetmocks.Provider{}
		provider.On("RetrieveFrom", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(client, nil)

		// when
		err := DeleteConflictedDefaultTag(ctx, provider, kubeConfig, log)

		// then
		require.NoError(t, err)
		taggedMwc, err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, taggedWhName, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, taggedMwcObj, taggedMwc)
	})
	t.Run("should not return an error if there is no tagged webhook", func(t *testing.T) {
		// given
		client := fake.NewSimpleClientset()
		provider := &clientsetmocks.Provider{}
		provider.On("RetrieveFrom", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(client, nil)

		// when
		err := DeleteConflictedDefaultTag(ctx, provider, kubeConfig, log)

		// then
		require.NoError(t, err)
	})
}
