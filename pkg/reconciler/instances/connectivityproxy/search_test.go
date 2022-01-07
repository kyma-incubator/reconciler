package connectivityproxy

import (
	"context"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	mockKubernetes "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	mock2 "github.com/stretchr/testify/mock"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"testing"
)

func TestServiceInstancesFilter(t *testing.T) {

	instances := []unstructured.Unstructured{
		{
			Object: map[string]interface{}{
				"apiVersion": "servicecatalog.k8s.io/v1beta1",
				"kind":       "ServiceInstance",
				"metadata": map[string]interface{}{
					"name":           "connectivity-virtuous-prompt",
					"namespace":      "default",
					"different-type": true,
					"nil-value":      nil,
				},
				"spec": map[string]interface{}{
					"clusterServiceClassExternalName": "connectivity",
				},
			},
		},
	}

	bindings := []unstructured.Unstructured{
		{
			Object: map[string]interface{}{
				"apiVersion": "servicecatalog.k8s.io/v1beta1",
				"kind":       "ServiceBinding",
				"metadata": map[string]interface{}{
					"name":      "sweet-kepler",
					"namespace": "default",
				},
				"spec": map[string]interface{}{
					"secretName": "sweet-kepler",
					"instanceRef": map[string]interface{}{
						"name": "connectivity-virtuous-prompt",
					},
				},
			},
		},
		{
			Object: map[string]interface{}{
				"apiVersion": "servicecatalog.k8s.io/v1beta1",
				"kind":       "ServiceBinding",
				"metadata": map[string]interface{}{
					"name":      "sweet-kepler-with-nil",
					"namespace": "default",
				},
				"spec": map[string]interface{}{
					"secretName": nil,
					"instanceRef": map[string]interface{}{
						"name": "connectivity-virtuous-prompt",
					},
				},
			},
		},
	}

	client := &mockKubernetes.Client{}
	client.On("ListResource", context.TODO(), "serviceinstance", v1.ListOptions{}).
		Return(&unstructured.UnstructuredList{
			Items: instances,
		}, nil)

	client.On("ListResource", context.TODO(), "servicebinding", v1.ListOptions{}).
		Return(&unstructured.UnstructuredList{
			Items: bindings,
		}, nil)

	client.On("ListResource", context.TODO(), "test-invalid", v1.ListOptions{}).
		Return(nil, errors.New("Test error"))

	client.On("ListResource", context.TODO(), mock2.AnythingOfType("string"), v1.ListOptions{}).
		Return(nil, k8serr.NewNotFound(schema.GroupResource{}, "test-message"))

	t.Run("Should find service instance", func(t *testing.T) {

		locator := Locator{
			resource:       "serviceinstance",
			field:          "spec.clusterServiceClassExternalName",
			client:         client,
			referenceValue: "connectivity",
		}
		found, err := locator.find(context.TODO())

		assert.NoError(t, err)
		assert.Equal(t, &instances[0], found)
	})

	t.Run("Should find service instance by insensitive case", func(t *testing.T) {
		locator := Locator{
			resource:       "ServiceInstance",
			field:          "spec.clusterServiceClassExternalName",
			client:         client,
			referenceValue: "connectivity",
		}
		found, err := locator.find(context.TODO())

		assert.NoError(t, err)
		assert.Equal(t, &instances[0], found)
	})

	t.Run("Should return nil when non existing instance", func(t *testing.T) {
		locator := Locator{
			resource:       "ServiceInstanceNonExisting",
			field:          "spec.clusterServiceClassExternalName",
			client:         client,
			referenceValue: "connectivity",
		}
		_, err := locator.find(context.TODO())

		assert.NoError(t, err)
	})

	t.Run("Should propagate error from invalid list resources call", func(t *testing.T) {
		locator := Locator{
			resource:       "test-invalid",
			field:          "spec.clusterServiceClassExternalName",
			client:         client,
			referenceValue: "connectivity",
		}
		_, err := locator.find(context.TODO())

		assert.Error(t, err)
	})

	t.Run("Should return error on different value types", func(t *testing.T) {
		locator := Locator{
			resource:       "serviceinstance",
			field:          "metadata.different-type",
			client:         client,
			referenceValue: "true",
		}
		value, err := locator.find(context.TODO())

		assert.Error(t, err)
		assert.Nil(t, value)
	})

	t.Run("Should compare nil values", func(t *testing.T) {
		locator := Locator{
			resource:       "serviceinstance",
			field:          "metadata.nil-value",
			client:         client,
			referenceValue: nil,
		}
		value, err := locator.find(context.TODO())

		assert.NoError(t, err)
		assert.Equal(t, &instances[0], value)
	})

	t.Run("Should return nil for empty or nil criteria", func(t *testing.T) {
		s := Search{}

		result, err := s.findByCriteria(context.TODO(), []Locator{})
		assert.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("Should return first result for one item criteria", func(t *testing.T) {
		s := Search{}

		result, err := s.findByCriteria(context.TODO(), []Locator{
			{
				referenceValue: "connectivity",
				resource:       "serviceinstance",
				field:          "spec.clusterServiceClassExternalName",
				client:         client,
			},
		})

		assert.NoError(t, err)
		assert.Equal(t, &instances[0], result)
	})

	t.Run("Should return nil if criteria not found ", func(t *testing.T) {
		s := Search{}

		result, err := s.findByCriteria(context.TODO(), []Locator{
			{
				referenceValue: "non-existing-value",
				resource:       "serviceinstance",
				field:          "spec.clusterServiceClassExternalName",
				client:         client,
			},
		})

		assert.NoError(t, err)
		assert.Nil(t, result)

		result, err = s.findByCriteria(context.TODO(), []Locator{
			{
				referenceValue: "non-existing-value",
				resource:       "serviceinstance",
				field:          "spec.clusterServiceClassExternalName",
				client:         client,
				searchNextBy:   "metadata.name",
			},
			{
				resource:     "servicebinding",
				field:        "spec.instanceRef.name",
				client:       client,
				searchNextBy: "",
			},
		})

		assert.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("Should find by combined criteria", func(t *testing.T) {
		s := Search{}

		result, err := s.findByCriteria(context.TODO(), []Locator{
			{
				referenceValue: "connectivity",
				resource:       "serviceinstance",
				field:          "spec.clusterServiceClassExternalName",
				client:         client,
				searchNextBy:   "metadata.name",
			},
			{
				resource:     "servicebinding",
				field:        "spec.instanceRef.name",
				client:       client,
				searchNextBy: "",
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, &bindings[0], result)
	})

	t.Run("Should find by nil value", func(t *testing.T) {
		s := Search{}

		result, err := s.findByCriteria(context.TODO(), []Locator{
			{
				referenceValue: nil,
				resource:       "servicebinding",
				field:          "spec.secretName",
				client:         client,
				searchNextBy:   "metadata.name",
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, &bindings[1], result)
	})
}
