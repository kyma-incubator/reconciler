package reset

import (
	"sync"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/pod"

	v1 "k8s.io/api/core/v1"
)

//go:generate mockery -name=Action -outpkg=mocks -case=underscore
type Action interface {
	Reset(podsList v1.PodList)
}

// DefaultResetAction assigns pods to handlers and executes them
type DefaultResetAction struct {
	matcher pod.Matcher
	wg      sync.WaitGroup
}

func NewDefaultPodsResetAction(matcher pod.Matcher) *DefaultResetAction {
	return &DefaultResetAction{
		matcher: matcher,
		wg:      sync.WaitGroup{},
	}
}

func (i *DefaultResetAction) Reset(podsList v1.PodList) {
	handlersMap := i.matcher.GetHandlersMap(podsList)

	for handler := range handlersMap {
		for _, object := range handlersMap[handler] {
			i.wg.Add(1)
			go handler.Execute(object, &i.wg)
		}
	}

	i.wg.Wait()
}
