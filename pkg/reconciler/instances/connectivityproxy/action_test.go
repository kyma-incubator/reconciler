package connectivityproxy

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAction(t *testing.T) {
	t.Run("Should invoke operations", func(t *testing.T) {
		t.Skip()

		expected := v1.Secret{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
			Immutable: nil,
			Data: map[string][]byte{
				"token": []byte("tokenValue"),
			},
			StringData: nil,
			Type:       "",
		}

		invoked := 0
		action := CustomAction{
			name: "testAction",
			copyFactory: []CopyFactory{
				func(configs map[string]interface{}, inClusterClientSet, targetClientSet k8s.Interface) *SecretCopy {
					invoked++
					return &SecretCopy{
						Namespace:       "namespace",
						Name:            "name",
						targetClientSet: fake.NewSimpleClientset(),
						from: &FromSecret{
							Namespace: "test-namespace",
							Name:      "test-name",
							inCluster: fake.NewSimpleClientset(&expected),
						},
					}
				},
				func(configs map[string]interface{}, inClusterClientSet, targetClientSet k8s.Interface) *SecretCopy {
					invoked++
					return &SecretCopy{
						Namespace:       "namespace",
						Name:            "name",
						targetClientSet: fake.NewSimpleClientset(),
						from: &FromSecret{
							Namespace: "test-namespace",
							Name:      "test-name",
							inCluster: fake.NewSimpleClientset(&expected),
						},
					}
				},
			},
		}

		client := mocks.Client{}
		client.On("Clientset").Return(fake.NewSimpleClientset(), nil)

		err := action.Run(&service.ActionContext{
			KubeClient:       &client,
			WorkspaceFactory: nil,
			Context:          nil,
			Logger:           nil,
			ChartProvider:    nil,
			Model:            nil,
		})

		require.NoError(t, err)
		require.Equal(t, 2, invoked)
	})
}
