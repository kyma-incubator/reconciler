package reset

import (
	"sync"

	"github.com/avast/retry-go"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/pod"

	v1 "k8s.io/api/core/v1"
)

//go:generate mockery --name=Action --outpkg=mocks --case=underscore
type Action interface {
	Reset(kubeClient kubernetes.Interface, retryOpts []retry.Option, podsList v1.PodList, log *zap.SugaredLogger, debug bool, waitOpts pod.WaitOptions)
}

// DefaultResetAction assigns pods to handlers and executes them
type DefaultResetAction struct {
	matcher pod.Matcher
	wg      *sync.WaitGroup
}

func NewDefaultPodsResetAction(matcher pod.Matcher) *DefaultResetAction {
	waitGroup := sync.WaitGroup{}

	return &DefaultResetAction{
		matcher: matcher,
		wg:      &waitGroup,
	}
}

func (i *DefaultResetAction) GetWG() *sync.WaitGroup {
	return i.wg
}

func (i *DefaultResetAction) Reset(kubeClient kubernetes.Interface, retryOpts []retry.Option, podsList v1.PodList, log *zap.SugaredLogger, debug bool, waitOpts pod.WaitOptions) {
	handlersMap := i.matcher.GetHandlersMap(kubeClient, retryOpts, podsList, log, debug, waitOpts)

	for handler := range handlersMap {
		for _, object := range handlersMap[handler] {
			i.wg.Add(1)
			handler.Execute(object, i.GetWG)
			i.wg.Add(1)
			err := handler.WaitForResources(object, i.GetWG)
			if err != nil {
				log.Error(err)
			}
		}
	}

	i.GetWG().Wait()
}
