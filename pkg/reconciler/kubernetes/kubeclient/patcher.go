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
	"k8s.io/kubectl/pkg/util/openapi"
)

const (
	// maxPatchRetry is the maximum number of conflicts retry for during a patch operation before returning failure
	maxPatchRetry = 5
	// // backOffPeriod is the period to back off when kubeClient patch results in error.
	// backOffPeriod = 1 * time.Second
	// // how many times we can retry before back off
	// triesBeforeBackOff = 1
)

var backoff = wait.Backoff{
	Steps:    3,
	Duration: 500 * time.Millisecond,
	Factor:   1.0,
	Jitter:   0.1,
}

func newPatcher(info *resource.Info, helper *resource.Helper) *Patcher {
	var openapiSchema openapi.Resources

	return &Patcher{
		Helper:        helper,
		Overwrite:     true,
		Force:         false,
		Cascade:       true,
		Timeout:       time.Duration(0),
		GracePeriod:   -1,
		OpenapiSchema: openapiSchema,
	}
}

type Patcher struct {
	Helper    *resource.Helper
	Overwrite bool

	Force       bool
	Cascade     bool
	Timeout     time.Duration
	GracePeriod int

	// If set, forces the patch against a specific resourceVersion
	ResourceVersion *string

	OpenapiSchema openapi.Resources
}

func (p *Patcher) replaceObj(new runtime.Object, namespace, name string) (runtime.Object, error) {
	var result runtime.Object
	var err error

	err = wait.ExponentialBackoff(backoff, func() (bool, error) {
		result, err = p.Helper.Replace(namespace, name, true, new)
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

func (p *Patcher) getResourceVersion(namespace, name string) (string, error) {
	var getResult runtime.Object
	var err error

	err = wait.ExponentialBackoff(backoff, func() (done bool, err error) {
		getResult, err = p.Helper.Get(namespace, name)

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

func (p *Patcher) delete(namespace, name string) error {
	options := asDeleteOptions(p.Cascade, p.GracePeriod)
	err := wait.ExponentialBackoff(backoff, func() (done bool, err error) {
		if _, err := p.Helper.DeleteWithOptions(namespace, name, &options); err != nil {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return err
	}

	return wait.PollImmediate(time.Second, p.Timeout, func() (done bool, err error) {
		if _, err := p.Helper.Get(namespace, name); !errors.IsNotFound(err) {
			return false, err
		}
		return true, nil
	})
}

func (p *Patcher) createObj(obj runtime.Object, namespace string) (runtime.Object, error) {
	var result runtime.Object
	err := wait.ExponentialBackoff(backoff, func() (done bool, err error) {
		result, err = p.Helper.Create(namespace, true, obj)
		if err != nil {
			return false, err
		}
		return true, nil
	})
	return result, err
}

func (p *Patcher) recreateObject(obj runtime.Object, namespace, name string) (runtime.Object, error) {
	// try to delete resource
	if err := p.delete(namespace, name); err != nil {
		return nil, err
	}

	return p.createObj(obj, namespace)
}

func (p *Patcher) replace(new runtime.Object, namespace, name string) (result runtime.Object, err error) {
	for i := 0; i < maxPatchRetry; i++ {
		result, err = p.simpleReplace(new, namespace, name)

		if err == nil {
			return result, err
		}

		if errors.IsConflict(err) && p.ResourceVersion != nil {
			break
		}

		if !errors.IsConflict(err) {
			break
		}
	}

	if err != nil && (errors.IsConflict(err) || errors.IsInvalid(err)) && p.Force {
		return p.recreateObject(new, namespace, name)
	}

	return
}

func (p *Patcher) simpleReplace(new runtime.Object, namespace, name string) (runtime.Object, error) {
	// prepare new resource configuration
	newMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(new)
	if err != nil {
		return nil, err
	}

	newU := unstructured.Unstructured{Object: newMap}

	// update resource version
	if p.ResourceVersion != nil {
		newU.SetResourceVersion(*p.ResourceVersion)
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
