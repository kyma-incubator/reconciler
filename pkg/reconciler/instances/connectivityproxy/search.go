package connectivityproxy

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/pkg/errors"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"reflect"
	"strings"
)

const sep = "."

type Search struct {
}

type Locator struct {
	searchNextBy   string
	referenceValue interface{}
	resource       string
	field          string
	client         kubernetes.Client
}

func (si *Search) findByCriteria(criteria []Locator) (*unstructured.Unstructured, error) {
	if len(criteria) == 0 {
		return nil, nil
	}

	crit := criteria[0]
	result, err := crit.find()
	if err != nil {
		return nil, err
	}

	if len(criteria) == 1 {
		return result, nil
	}

	if result != nil && len(criteria) > 1 {
		fields := strings.Split(criteria[0].searchNextBy, sep)

		var result Map = result.Object
		criteria[1].referenceValue = result.getValue(fields...)
		return si.findByCriteria(criteria[1:])
	}

	return nil, nil
}

func (c *Locator) find() (*unstructured.Unstructured, error) {
	resources, err := c.client.ListResource(strings.ToLower(c.resource), metav1.ListOptions{})

	if err != nil && k8serr.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, errors.Wrap(err, "Error while listing resources")
	}

	fields := strings.Split(c.field, sep)

	for _, item := range resources.Items {
		obj := item.Object
		var uns Map = obj
		currentValue := uns.getValue(fields...)

		if currentValue != nil && c.referenceValue != nil &&
			reflect.TypeOf(currentValue) != reflect.TypeOf(c.referenceValue) {
			return nil, errors.New("Invalid types")
		}

		if currentValue == c.referenceValue {
			return &item, nil
		}
	}

	return nil, nil
}
