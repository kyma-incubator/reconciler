package kubernetes

import (
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"os"
	"path/filepath"
	"testing"
)

func TestResourceList(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("test", "unittest-with-namespace.yaml"))
	require.NoError(t, err)

	unstructs, err := ToUnstructured(data, true)
	require.NoError(t, err)

	resources := NewResourceList(unstructs)

	t.Run("Test length", func(t *testing.T) {
		require.Equal(t, 5, resources.Len())
	})

	t.Run("Test visitor without error", func(t *testing.T) {
		var counter int
		err := resources.Visit(func(u *unstructured.Unstructured) error {
			counter++
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, 5, counter)
	})

	t.Run("Test visitor by kind", func(t *testing.T) {
		var counter int
		err := resources.VisitByKind("Deployment", func(u *unstructured.Unstructured) error {
			counter++
			require.Equal(t, u.GetKind(), "Deployment")
			require.Equal(t, u.GetName(), "unittest-deployment")
			require.Equal(t, u.GetNamespace(), "unittest-adapter")
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, 1, counter)
	})

	t.Run("Test visitor with error", func(t *testing.T) {
		var counter int
		err := resources.Visit(func(u *unstructured.Unstructured) error {
			counter++
			if counter >= 2 {
				return errors.New("fake error")
			}
			return nil
		})
		require.Error(t, err)
		require.Equal(t, 2, counter)
	})

	t.Run("Test get by kind", func(t *testing.T) {
		depResources := resources.GetByKind("deployment")
		require.Len(t, depResources, 1)
		require.Equal(t, "unittest-deployment", depResources[0].GetName())
	})

	t.Run("Test get", func(t *testing.T) {
		u := resources.Get("Deployment", "unittest-deployment", "unittest-adapter")
		require.NotEmpty(t, u)
		require.Equal(t, "Deployment", u.GetKind())
		require.Equal(t, "unittest-deployment", u.GetName())
		require.Equal(t, "unittest-adapter", u.GetNamespace())
	})

	t.Run("Test get with undefined namespace", func(t *testing.T) {
		newU := &unstructured.Unstructured{}
		newU.SetKind("Fake")
		newU.SetName("fake")
		resources.Add(newU)
		require.Len(t, resources.GetByKind("Fake"), 1)
		u := resources.Get("Fake", "fake", "unittest-adapter")
		require.Equal(t, "Fake", u.GetKind())
		require.Equal(t, "fake", u.GetName())
		require.Equal(t, "", u.GetNamespace())
		resources.Remove(newU)
		require.Len(t, resources.GetByKind("Fake"), 0)
	})

	t.Run("Test add and remove", func(t *testing.T) {
		u1 := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"kind": "service",
				"metadata": map[string]interface{}{
					"name":      "service-1",
					"namespace": "service-ns1",
				},
			},
		}
		u2 := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"kind": "service",
				"metadata": map[string]interface{}{
					"name":      "service-2",
					"namespace": "service-ns2",
				},
			},
		}
		require.Len(t, resources.resources, 5)
		require.Len(t, resources.GetByKind("service"), 0)
		resources.Add(u1)
		resources.Add(u2)
		require.Len(t, resources.resources, 7)
		require.Len(t, resources.GetByKind("service"), 2)
		resources.Remove(u1)
		require.Len(t, resources.resources, 6)
		require.Len(t, resources.GetByKind("service"), 1)
		resources.Remove(u2)
		require.Len(t, resources.resources, 5)
		require.Len(t, resources.GetByKind("service"), 0)
	})

	t.Run("Test replace", func(t *testing.T) {
		u1a := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"kind": "service",
				"metadata": map[string]interface{}{
					"name": "service-1",
					"labels": map[string]interface{}{
						"test": "label-old",
					},
				},
			},
		}
		u1b := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"kind": "service",
				"metadata": map[string]interface{}{
					"name": "service-1",
					"labels": map[string]interface{}{
						"test": "label-new",
					},
				},
			},
		}
		u2 := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"kind": "service",
				"metadata": map[string]interface{}{
					"name": "service-2",
				},
			},
		}
		resources.Add(u1a)
		resources.Add(u2)
		require.Len(t, resources.resources, 7)
		require.Equal(t, map[string]string{"test": "label-old"}, resources.Get("service", "service-1", "").GetLabels())
		resources.Replace(u1b)
		require.Len(t, resources.resources, 7)
		require.Equal(t, map[string]string{"test": "label-new"}, resources.Get("service", "service-1", "").GetLabels())
		resources.Remove(u1b)
		resources.Remove(u2)
	})

}
