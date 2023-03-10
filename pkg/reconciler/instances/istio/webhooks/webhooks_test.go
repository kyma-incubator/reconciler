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

var validSelector = &metav1.LabelSelector{
	MatchExpressions: []metav1.LabelSelectorRequirement{{
		Key:      "istio-injection",
		Operator: "DoesNotExist",
	}},
}

func createDefaultMutatingWebhookWithSelector(whConfName string, labelKey string, selector *metav1.LabelSelector) *v1.MutatingWebhookConfiguration {
	return &v1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   whConfName,
			Labels: map[string]string{labelKey: tag.DefaultRevisionName},
		},
		Webhooks: []v1.MutatingWebhook{
			{
				Name:              "object.sidecar-injector.istio.io",
				NamespaceSelector: selector,
			},
		},
	}
}

func Test_DeleteConflictDefaultTag(t *testing.T) {
	kubeConfig := "kubeconfig"
	log := logger.NewLogger(false)
	ctx := context.Background()
	oldWhName := "istio-sidecar-injector"
	taggedWhName := "istio-revision-tag-default"

	defer ctx.Done()

	t.Run("should return error when kubeclient could not be retrieved", func(t *testing.T) {
		// given
		provider := clientsetmocks.Provider{}
		provider.On("RetrieveFrom", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(nil, errors.New("Kubernetes client error"))

		// when
		err := DeleteConflictDefaultTag(ctx, &provider, kubeConfig, log)

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Kubernetes client error")
	})
	t.Run("should not delete tagged webhook when old webhook is deactivated", func(t *testing.T) {
		// given
		wh := createDefaultMutatingWebhookWithSelector(oldWhName, "istio.io/rev", deactivatedSelector)
		wh2 := createDefaultMutatingWebhookWithSelector(taggedWhName, tag.IstioTagLabel, validSelector)
		client := fake.NewSimpleClientset(wh, wh2)
		provider := &clientsetmocks.Provider{}
		provider.On("RetrieveFrom", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(client, nil)
		// when
		err := DeleteConflictDefaultTag(ctx, provider, kubeConfig, log)

		// then
		require.NoError(t, err)
		got, err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, oldWhName, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, wh, got)
	})
	t.Run("should delete conflicted tagged webhook if old one is not deactivated", func(t *testing.T) {
		// given
		wh := createDefaultMutatingWebhookWithSelector(oldWhName, "istio.io/rev", validSelector)
		wh2 := createDefaultMutatingWebhookWithSelector(taggedWhName, tag.IstioTagLabel, validSelector)
		client := fake.NewSimpleClientset(wh, wh2)
		provider := &clientsetmocks.Provider{}
		provider.On("RetrieveFrom", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(client, nil)
		// when
		err := DeleteConflictDefaultTag(ctx, provider, kubeConfig, log)

		// then
		require.NoError(t, err)
		got, err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, taggedWhName, metav1.GetOptions{})
		require.ErrorContains(t, err, "not found")
		require.Nil(t, got)
	})
	t.Run("should not delete tagged webhook if there is no old webhook", func(t *testing.T) {
		// given
		wh := createDefaultMutatingWebhookWithSelector(taggedWhName, tag.IstioTagLabel, validSelector)
		client := fake.NewSimpleClientset(wh)
		provider := &clientsetmocks.Provider{}
		provider.On("RetrieveFrom", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(client, nil)
		// when
		err := DeleteConflictDefaultTag(ctx, provider, kubeConfig, log)

		// then
		require.NoError(t, err)
		got, err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, taggedWhName, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, wh, got)
	})
	t.Run("should not return an error if there is no tagged webhook", func(t *testing.T) {
		// given
		wh := createDefaultMutatingWebhookWithSelector(oldWhName, "istio.io/rev", validSelector)
		client := fake.NewSimpleClientset(wh)
		provider := &clientsetmocks.Provider{}
		provider.On("RetrieveFrom", mock.AnythingOfType("string"), mock.AnythingOfType("*zap.SugaredLogger")).Return(client, nil)
		// when
		err := DeleteConflictDefaultTag(ctx, provider, kubeConfig, log)

		// then
		require.NoError(t, err)
		got, err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, oldWhName, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, wh, got)
	})
}
