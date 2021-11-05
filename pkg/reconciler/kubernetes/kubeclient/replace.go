// solution from https://github.com/billiford/go-clouddriver/blob/master/pkg/kubernetes/patcher/patcher.go

package kubeclient

import (
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/resource"
)

const (
	maxPatchRetry = 5
)

var defaultBackoff = wait.Backoff{
	Steps:    3,
	Duration: 500 * time.Millisecond,
	Factor:   1.0,
	Jitter:   0.1,
}

func newReplace(helper *resource.Helper) replace {
	r := replacer{
		helper:      helper,
		overwrite:   true,
		force:       false,
		cascade:     true,
		timeout:     time.Duration(0),
		gracePeriod: -1,
	}
	return r.replace
}

type replace func(new runtime.Object, namespace, name string) (result runtime.Object, err error)

type replacer struct {
	helper    *resource.Helper
	overwrite bool

	force       bool
	cascade     bool
	timeout     time.Duration
	gracePeriod int

	// If set, forces the patch against a specific resourceVersion
	resourceVersion *string
}

func (p *replacer) replaceObj(new runtime.Object, namespace, name string) (runtime.Object, error) {
	var result runtime.Object
	var err error

	err = wait.ExponentialBackoff(defaultBackoff, func() (bool, error) {
		result, err = p.helper.Replace(namespace, name, p.overwrite, new)
		// detect unretryable errors
		if errors.IsConflict(err) || errors.IsInvalid(err) {
			return true, err
		}
		// retry if error
		if err != nil {
			return false, err
		}
		// replace is done
		return true, nil
	})

	return result, err
}

func (p *replacer) getResourceVersion(namespace, name string) (string, error) {
	var getResult runtime.Object
	var err error

	err = wait.ExponentialBackoff(defaultBackoff, func() (done bool, err error) {
		getResult, err = p.helper.Get(namespace, name)

		if errors.IsNotFound(err) {
			return true, err
		}

		if err != nil {
			return false, err
		}

		return true, nil
	})

	// fail if error cannot be recovered
	if err != nil {
		return "", err
	}

	resMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(getResult)
	if err != nil {
		return "", err
	}
	resU := unstructured.Unstructured{Object: resMap}
	return resU.GetResourceVersion(), nil
}

func (p *replacer) delete(namespace, name string) error {
	options := asDeleteOptions(p.cascade, p.gracePeriod)
	err := wait.ExponentialBackoff(defaultBackoff, func() (done bool, err error) {
		if _, err := p.helper.DeleteWithOptions(namespace, name, &options); err != nil {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return err
	}

	return wait.PollImmediate(time.Second, p.timeout, func() (done bool, err error) {
		if _, err := p.helper.Get(namespace, name); !errors.IsNotFound(err) {
			return false, err
		}
		return true, nil
	})
}

func (p *replacer) createObj(obj runtime.Object, namespace string) (runtime.Object, error) {
	var result runtime.Object
	err := wait.ExponentialBackoff(defaultBackoff, func() (done bool, err error) {
		result, err = p.helper.Create(namespace, true, obj)
		if err != nil {
			return false, err
		}
		return true, nil
	})
	return result, err
}

func (p *replacer) recreateObject(obj runtime.Object, namespace, name string) (runtime.Object, error) {
	// try to delete resource
	if err := p.delete(namespace, name); err != nil {
		return nil, err
	}

	return p.createObj(obj, namespace)
}

func (p *replacer) replace(new runtime.Object, namespace, name string) (result runtime.Object, err error) {
	for i := 0; i < maxPatchRetry; i++ {
		result, err = p.simpleReplace(new, namespace, name)

		if err == nil {
			return result, err
		}

		if errors.IsConflict(err) && p.resourceVersion != nil {
			break
		}

		if !errors.IsConflict(err) {
			break
		}
	}

	if err != nil && (errors.IsConflict(err) || errors.IsInvalid(err)) && p.force {
		return p.recreateObject(new, namespace, name)
	}

	return
}

func (p *replacer) simpleReplace(new runtime.Object, namespace, name string) (runtime.Object, error) {
	// prepare new resource configuration
	newMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(new)
	if err != nil {
		return nil, err
	}

	newU := unstructured.Unstructured{Object: newMap}

	// update resource version
	if p.resourceVersion != nil {
		newU.SetResourceVersion(*p.resourceVersion)
	} else {
		resourceVersion, err := p.getResourceVersion(namespace, name)
		if err != nil {
			return nil, err
		}
		newU.SetResourceVersion(resourceVersion)
	}

	// try to replace object
	return p.replaceObj(&newU, namespace, name)
}

func asDeleteOptions(cascade bool, gracePeriod int) metav1.DeleteOptions {
	options := metav1.DeleteOptions{}
	if gracePeriod >= 0 {
		options = *metav1.NewDeleteOptions(int64(gracePeriod))
	}

	policy := metav1.DeletePropagationForeground
	if !cascade {
		policy = metav1.DeletePropagationOrphan
	}

	options.PropagationPolicy = &policy

	return options
}
