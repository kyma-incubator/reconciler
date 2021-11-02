// solution from https://github.com/billiford/go-clouddriver/blob/master/pkg/kubernetes/patcher/patcher.go

package kubeclient

import (
	"time"

	"github.com/jonboulle/clockwork"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
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
		Mapping:       info.Mapping,
		Helper:        helper,
		Overwrite:     true,
		BackOff:       clockwork.NewRealClock(),
		Force:         false,
		Cascade:       true,
		Timeout:       time.Duration(0),
		GracePeriod:   -1,
		OpenapiSchema: openapiSchema,
		Retries:       0,
	}
}

type Patcher struct {
	Mapping *meta.RESTMapping
	Helper  *resource.Helper

	Overwrite bool
	BackOff   clockwork.Clock

	Force       bool
	Cascade     bool
	Timeout     time.Duration
	GracePeriod int

	// If set, forces the patch against a specific resourceVersion
	ResourceVersion *string

	// Number of retries to make if the patch fails with conflict
	Retries int

	OpenapiSchema openapi.Resources
}

func (p *Patcher) replace(new runtime.Object, namespace, name string) (runtime.Object, error) {
	patchBytes, err := p.Helper.Replace(namespace, name, true, new)

	if p.Retries == 0 {
		p.Retries = maxPatchRetry
	}

	for i := 1; i <= p.Retries && errors.IsConflict(err); i++ {
		if i > triesBeforeBackOff {
			p.BackOff.Sleep(backOffPeriod)
		}
		patchBytes, err = p.Helper.Replace(namespace, name, true, new)
	}

	return patchBytes, err
}
