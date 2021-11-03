// solution from https://github.com/billiford/go-clouddriver/blob/master/pkg/kubernetes/patcher/patcher.go

package kubeclient

import (
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/kubectl/pkg/util/openapi"
)

const (
	// maxPatchRetry is the maximum number of conflicts retry for during a patch operation before returning failure
	maxPatchRetry = 5
	// backOffPeriod is the period to back off when kubeClient patch results in error.
	backOffPeriod = 1 * time.Second
	// how many times we can retry before back off
	triesBeforeBackOff = 1
)

func newPatcher(info *resource.Info, helper *resource.Helper) *Patcher {
	var openapiSchema openapi.Resources

	return &Patcher{
		Mapping:   info.Mapping,
		Helper:    helper,
		Overwrite: true,
		Backoff: wait.Backoff{
			Steps:    maxPatchRetry,
			Duration: backOffPeriod,
			Factor:   1.0,
			Jitter:   0.1,
		},
		Force:         false,
		Cascade:       true,
		Timeout:       time.Duration(0),
		GracePeriod:   -1,
		OpenapiSchema: openapiSchema,
	}
}

type Patcher struct {
	Mapping *meta.RESTMapping
	Helper  *resource.Helper

	Overwrite bool
	wait.Backoff

	Force       bool
	Cascade     bool
	Timeout     time.Duration
	GracePeriod int

	// If set, forces the patch against a specific resourceVersion
	ResourceVersion *string

	OpenapiSchema openapi.Resources
}

func (p *Patcher) replace(new runtime.Object, namespace, name string) (runtime.Object, error) {
	// try to replace object
	var result runtime.Object
	var err error

	err = wait.ExponentialBackoff(p.Backoff, func() (bool, error) {
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
	// object was replaced with no errors
	if err == nil {
		return result, nil
	}
	// fail if error cannot be recovered
	if errors.IsInvalid(err) {
		return nil, err
	}
	// get current configuration
	var current runtime.Object
	err = wait.ExponentialBackoff(p.Backoff, func() (bool, error) {
		current, err = p.Helper.Get(namespace, name)
		if err != nil {
			return false, err
		}
		return true, nil
	})
	// deletion failed
	if err != nil {
		return nil, err
	}
	// try to delete current object
	err = wait.ExponentialBackoff(p.Backoff, func() (bool, error) {
		options := asDeleteOptions(p.Cascade, p.GracePeriod)

		_, err := p.Helper.DeleteWithOptions(namespace, name, &options)
		if err != nil {
			return false, err
		}

		return true, nil
	})
	// deletion failed
	if err != nil {
		return nil, err
	}
	// wait deleted
	err = wait.PollImmediate(time.Second, time.Duration(0), func() (bool, error) {
		_, err := p.Helper.Get(namespace, name)
		if errors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})

	if err != nil {
		return nil, err
	}

	result, err = p.Helper.Create(namespace, true, new)
	if err == nil {
		return result, nil
	}
	// Retrieve the original configuration of the object from the annotation.
	return p.Helper.Create(namespace, true, current)
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

// func (p *Patcher) deleteAndCreate(original runtime.Object, modified []byte, namespace, name string) ([]byte, runtime.Object, error) {
// 	if err := p.delete(namespace, name); err != nil {
// 		return modified, nil, err
// 	}
// 	// TODO: use wait
// 	if err := wait.PollImmediate(1*time.Second, p.Timeout, func() (bool, error) {
// 		if _, err := p.Helper.Get(namespace, name); !errors.IsNotFound(err) {
// 			return false, err
// 		}

// 		return true, nil
// 	}); err != nil {
// 		return modified, nil, err
// 	}

// 	versionedObject, _, err := unstructured.UnstructuredJSONScheme.Decode(modified, nil, nil)
// 	if err != nil {
// 		return modified, nil, err
// 	}

// 	createdObject, err := p.Helper.Create(namespace, true, versionedObject)
// 	if err != nil {
// 		// restore the original object if we fail to create the new one
// 		// but still propagate and advertise error to user
// 		recreated, recreateErr := p.Helper.Create(namespace, true, original)
// 		if recreateErr != nil {
// 			err = fmt.Errorf("An error occurred force-replacing the existing object with the newly provided one:\n\n%v.\n\nAdditionally, an error occurred attempting to restore the original object:\n\n%v", err, recreateErr)
// 		} else {
// 			createdObject = recreated
// 		}
// 	}

// 	return modified, createdObject, err
// }
