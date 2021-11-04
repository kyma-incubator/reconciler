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
	Reset(kubeClient kubernetes.Interface, retryOpts []retry.Option, podsList v1.PodList, log *zap.SugaredLogger, debug bool, waitOpts pod.WaitOptions) error
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

func (i *DefaultResetAction) Reset(kubeClient kubernetes.Interface, retryOpts []retry.Option, podsList v1.PodList, log *zap.SugaredLogger, debug bool, waitOpts pod.WaitOptions) error {
	handlersMap := i.matcher.GetHandlersMap(kubeClient, retryOpts, podsList, log, debug, waitOpts)
	errorCh := make(chan error)

	for handler := range handlersMap {
		for _, object := range handlersMap[handler] {
			i.wg.Add(1)
			go func(object pod.CustomObject, wg *sync.WaitGroup) {
				handler.Execute(object, i.GetWG)
				err := handler.WaitForResources(object, i.GetWG)
				if err != nil {
					errorCh <- err
					return
				}
			}(object, i.wg)
		}
	}

	i.GetWG().Wait()
	close(errorCh)

	err := <-errorCh
	if err != nil {
		return err
	}

	return nil
}
