// Code generated by mockery 2.9.4. DO NOT EDIT.

package mocks

import (
	pod "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/pod"
	mock "github.com/stretchr/testify/mock"
)

// Handler is an autogenerated mock type for the Handler type
type Handler struct {
	mock.Mock
}

// Execute provides a mock function with given fields: _a0
func (_m *Handler) Execute(_a0 pod.CustomObject) {
	_m.Called(_a0)
}

// WaitForResources provides a mock function with given fields: _a0, _a1
func (_m *Handler) WaitForResources(_a0 pod.CustomObject, _a1 pod.GetSyncWG) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(pod.CustomObject, pod.GetSyncWG) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
