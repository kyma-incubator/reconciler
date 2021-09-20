package kubeclient

import (
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/resource"
	"testing"
)

func TestToUnstructured(t *testing.T) {
	t.Run("Should set namespace if scoped", func(t *testing.T) {
		u := &unstructured.Unstructured{}
		SetNamespaceIfScoped("test", u, &resource.Helper{
			NamespaceScoped: true,
		})
		require.Equal(t, "test", u.GetNamespace())
	})

	t.Run("Ignore namespace if not scoped", func(t *testing.T) {
		u := &unstructured.Unstructured{}
		u.SetNamespace("")
		SetNamespaceIfScoped("test", u, &resource.Helper{
			NamespaceScoped: false,
		})
		require.Equal(t, "", u.GetNamespace())

		u.SetNamespace("initial")
		SetNamespaceIfScoped("test", u, &resource.Helper{
			NamespaceScoped: false,
		})
		require.Equal(t, "initial", u.GetNamespace())

	})
}
