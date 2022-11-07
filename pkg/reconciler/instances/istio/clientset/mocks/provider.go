// Code generated by mockery v2.14.0. DO NOT EDIT.

package mock

import (
	client "sigs.k8s.io/controller-runtime/pkg/client"

	kubernetes "k8s.io/client-go/kubernetes"

	mock "github.com/stretchr/testify/mock"

	zap "go.uber.org/zap"
)

// Provider is an autogenerated mock type for the Provider type
type Provider struct {
	mock.Mock
}

// GetIstioClient provides a mock function with given fields: kubeConfig
func (_m *Provider) GetIstioClient(kubeConfig string) (client.Client, error) {
	ret := _m.Called(kubeConfig)

	var r0 client.Client
	if rf, ok := ret.Get(0).(func(string) client.Client); ok {
		r0 = rf(kubeConfig)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(client.Client)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(kubeConfig)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RetrieveFrom provides a mock function with given fields: kubeConfig, log
func (_m *Provider) RetrieveFrom(kubeConfig string, log *zap.SugaredLogger) (kubernetes.Interface, error) {
	ret := _m.Called(kubeConfig, log)

	var r0 kubernetes.Interface
	if rf, ok := ret.Get(0).(func(string, *zap.SugaredLogger) kubernetes.Interface); ok {
		r0 = rf(kubeConfig, log)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(kubernetes.Interface)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, *zap.SugaredLogger) error); ok {
		r1 = rf(kubeConfig, log)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewProvider interface {
	mock.TestingT
	Cleanup(func())
}

// NewProvider creates a new instance of Provider. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewProvider(t mockConstructorTestingTNewProvider) *Provider {
	mock := &Provider{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
