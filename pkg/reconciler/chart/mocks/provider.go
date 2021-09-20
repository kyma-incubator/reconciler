// Code generated by mockery 2.7.5. DO NOT EDIT.

package mock

import (
	chart "github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	mock "github.com/stretchr/testify/mock"
)

// Provider is an autogenerated mock type for the Provider type
type Provider struct {
	mock.Mock
}

// RenderCRD provides a mock function with given fields: version
func (_m *Provider) RenderCRD(version string) ([]*chart.Manifest, error) {
	ret := _m.Called(version)

	var r0 []*chart.Manifest
	if rf, ok := ret.Get(0).(func(string) []*chart.Manifest); ok {
		r0 = rf(version)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*chart.Manifest)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(version)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RenderManifest provides a mock function with given fields: component
func (_m *Provider) RenderManifest(component *chart.Component) (*chart.Manifest, error) {
	ret := _m.Called(component)

	var r0 *chart.Manifest
	if rf, ok := ret.Get(0).(func(*chart.Component) *chart.Manifest); ok {
		r0 = rf(component)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*chart.Manifest)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*chart.Component) error); ok {
		r1 = rf(component)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
