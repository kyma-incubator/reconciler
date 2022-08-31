// Code generated by mockery v2.14.0. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	kubernetes "k8s.io/client-go/kubernetes"

	pod "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/pod"

	retry "github.com/avast/retry-go"

	v1 "k8s.io/api/core/v1"

	wait "k8s.io/apimachinery/pkg/util/wait"

	zap "go.uber.org/zap"
)

// Action is an autogenerated mock type for the Action type
type Action struct {
	mock.Mock
}

// LabelWithWarning provides a mock function with given fields: _a0, kubeClient, retryOpts, podsList, log
func (_m *Action) LabelWithWarning(_a0 context.Context, kubeClient kubernetes.Interface, retryOpts wait.Backoff, podsList v1.PodList, log *zap.SugaredLogger) error {
	ret := _m.Called(_a0, kubeClient, retryOpts, podsList, log)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, kubernetes.Interface, wait.Backoff, v1.PodList, *zap.SugaredLogger) error); ok {
		r0 = rf(_a0, kubeClient, retryOpts, podsList, log)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Reset provides a mock function with given fields: _a0, kubeClient, retryOpts, podsList, log, debug, waitOpts
func (_m *Action) Reset(_a0 context.Context, kubeClient kubernetes.Interface, retryOpts []retry.Option, podsList v1.PodList, log *zap.SugaredLogger, debug bool, waitOpts pod.WaitOptions) error {
	ret := _m.Called(_a0, kubeClient, retryOpts, podsList, log, debug, waitOpts)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, kubernetes.Interface, []retry.Option, v1.PodList, *zap.SugaredLogger, bool, pod.WaitOptions) error); ok {
		r0 = rf(_a0, kubeClient, retryOpts, podsList, log, debug, waitOpts)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewAction interface {
	mock.TestingT
	Cleanup(func())
}

// NewAction creates a new instance of Action. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewAction(t mockConstructorTestingTNewAction) *Action {
	mock := &Action{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
