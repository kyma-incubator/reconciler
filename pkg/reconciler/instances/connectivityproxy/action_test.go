package connectivityproxy

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/stretchr/testify/require"
)

func TestAction(t *testing.T) {
	t.Run("Should invoke operations", func(t *testing.T) {
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
				func(context *service.ActionContext) *SecretCopy {
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
				func(context *service.ActionContext) *SecretCopy {
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

		err := action.Run("", "", nil, nil)

		require.NoError(t, err)
		require.Equal(t, 2, invoked)
	})
}
