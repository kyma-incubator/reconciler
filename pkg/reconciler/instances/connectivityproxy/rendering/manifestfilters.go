package rendering

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func NewFilterOutAnnotatedManifests(annotation string) FilterFunc {
	return func(unstructs []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
		newUnstructs := make([]*unstructured.Unstructured, 0)

		for _, unstruct := range unstructs {
			annotations := unstruct.GetAnnotations()
			_, ok := annotations[annotation]
			if !ok {
				newUnstructs = append(newUnstructs, unstruct)
			}
		}

		return newUnstructs, nil
	}
}
