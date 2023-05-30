// Code generated by mockery v2.24.0. DO NOT EDIT.

package connectivityproxymocks

import (
	connectivityclient "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/connectivityproxy/connectivityclient"

	mock "github.com/stretchr/testify/mock"

	service "github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

// Commands is an autogenerated mock type for the Commands type
type Commands struct {
	mock.Mock
}

// Apply provides a mock function with given fields: _a0, _a1
func (_m *Commands) Apply(_a0 *service.ActionContext, _a1 bool) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(*service.ActionContext, bool) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreateCARootSecret provides a mock function with given fields: _a0, _a1
func (_m *Commands) CreateCARootSecret(_a0 *service.ActionContext, _a1 connectivityclient.ConnectivityClient) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(*service.ActionContext, connectivityclient.ConnectivityClient) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreateSecretMappingOperator provides a mock function with given fields: _a0, _a1
func (_m *Commands) CreateSecretMappingOperator(_a0 *service.ActionContext, _a1 string) (map[string][]byte, error) {
	ret := _m.Called(_a0, _a1)

	var r0 map[string][]byte
	var r1 error
	if rf, ok := ret.Get(0).(func(*service.ActionContext, string) (map[string][]byte, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(*service.ActionContext, string) map[string][]byte); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[string][]byte)
		}
	}

	if rf, ok := ret.Get(1).(func(*service.ActionContext, string) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CreateServiceMappingConfigMap provides a mock function with given fields: ctx, ns, configMapName
func (_m *Commands) CreateServiceMappingConfigMap(ctx *service.ActionContext, ns string, configMapName string) error {
	ret := _m.Called(ctx, ns, configMapName)

	var r0 error
	if rf, ok := ret.Get(0).(func(*service.ActionContext, string, string) error); ok {
		r0 = rf(ctx, ns, configMapName)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Remove provides a mock function with given fields: _a0
func (_m *Commands) Remove(_a0 *service.ActionContext) error {
	ret := _m.Called(_a0)

	var r0 error
	if rf, ok := ret.Get(0).(func(*service.ActionContext) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewCommands interface {
	mock.TestingT
	Cleanup(func())
}

// NewCommands creates a new instance of Commands. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewCommands(t mockConstructorTestingTNewCommands) *Commands {
	mock := &Commands{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
