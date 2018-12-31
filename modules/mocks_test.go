// Code generated by MockGen. DO NOT EDIT.
// Source: modules.go

// Package modules_test is a generated GoMock package.
package modules_test

import (
	gomock "github.com/golang/mock/gomock"
	reflect "reflect"
)

// MockPackageManager is a mock of PackageManager interface
type MockPackageManager struct {
	ctrl     *gomock.Controller
	recorder *MockPackageManagerMockRecorder
}

// MockPackageManagerMockRecorder is the mock recorder for MockPackageManager
type MockPackageManagerMockRecorder struct {
	mock *MockPackageManager
}

// NewMockPackageManager creates a new mock instance
func NewMockPackageManager(ctrl *gomock.Controller) *MockPackageManager {
	mock := &MockPackageManager{ctrl: ctrl}
	mock.recorder = &MockPackageManagerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockPackageManager) EXPECT() *MockPackageManagerMockRecorder {
	return m.recorder
}

// InstallOffline mocks base method
func (m *MockPackageManager) InstallOffline(location string) error {
	ret := m.ctrl.Call(m, "InstallOffline", location)
	ret0, _ := ret[0].(error)
	return ret0
}

// InstallOffline indicates an expected call of InstallOffline
func (mr *MockPackageManagerMockRecorder) InstallOffline(location interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "InstallOffline", reflect.TypeOf((*MockPackageManager)(nil).InstallOffline), location)
}

// InstallOnline mocks base method
func (m *MockPackageManager) InstallOnline(location string) error {
	ret := m.ctrl.Call(m, "InstallOnline", location)
	ret0, _ := ret[0].(error)
	return ret0
}

// InstallOnline indicates an expected call of InstallOnline
func (mr *MockPackageManagerMockRecorder) InstallOnline(location interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "InstallOnline", reflect.TypeOf((*MockPackageManager)(nil).InstallOnline), location)
}
