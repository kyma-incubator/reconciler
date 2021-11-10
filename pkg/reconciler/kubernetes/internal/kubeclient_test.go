package internal

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/resource"
)

func TestSetNamespaceIfScoped(t *testing.T) {
	t.Run("Should set namespace if scoped", func(t *testing.T) {
		u := &unstructured.Unstructured{}
		setNamespaceIfScoped("test", u, &resource.Helper{
			NamespaceScoped: true,
		})
		require.Equal(t, "test", u.GetNamespace())
	})

	t.Run("Ignore namespace if not scoped", func(t *testing.T) {
		u := &unstructured.Unstructured{}
		u.SetNamespace("")
		setNamespaceIfScoped("test", u, &resource.Helper{
			NamespaceScoped: false,
		})
		require.Equal(t, "", u.GetNamespace())

		u.SetNamespace("initial")
		setNamespaceIfScoped("test", u, &resource.Helper{
			NamespaceScoped: false,
		})
		require.Equal(t, "initial", u.GetNamespace())

	})
}
