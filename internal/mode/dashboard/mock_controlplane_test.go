package dashboard

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/zjrosen/perles/internal/orchestration/controlplane"
)

// mockControlPlane is a test mock for controlplane.ControlPlane.
type mockControlPlane struct {
	mock.Mock
}

func newMockControlPlane() *mockControlPlane {
	m := &mockControlPlane{}
	// Default mock for GetHealthStatus - returns healthy status
	// Individual tests can override with more specific expectations
	m.On("GetHealthStatus", mock.Anything).Return(controlplane.HealthStatus{
		IsHealthy: true,
	}, true).Maybe()
	return m
}

func (m *mockControlPlane) Create(ctx context.Context, spec controlplane.WorkflowSpec) (controlplane.WorkflowID, error) {
	args := m.Called(ctx, spec)
	return args.Get(0).(controlplane.WorkflowID), args.Error(1)
}

func (m *mockControlPlane) Start(ctx context.Context, id controlplane.WorkflowID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockControlPlane) Stop(ctx context.Context, id controlplane.WorkflowID, opts controlplane.StopOptions) error {
	args := m.Called(ctx, id, opts)
	return args.Error(0)
}

func (m *mockControlPlane) Get(ctx context.Context, id controlplane.WorkflowID) (*controlplane.WorkflowInstance, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*controlplane.WorkflowInstance), args.Error(1)
}

func (m *mockControlPlane) List(ctx context.Context, q controlplane.ListQuery) ([]*controlplane.WorkflowInstance, error) {
	args := m.Called(ctx, q)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*controlplane.WorkflowInstance), args.Error(1)
}

func (m *mockControlPlane) Subscribe(ctx context.Context) (<-chan controlplane.ControlPlaneEvent, func()) {
	args := m.Called(ctx)
	ch := args.Get(0)
	if ch == nil {
		return nil, args.Get(1).(func())
	}
	return ch.(<-chan controlplane.ControlPlaneEvent), args.Get(1).(func())
}

func (m *mockControlPlane) SubscribeWorkflow(ctx context.Context, id controlplane.WorkflowID) (<-chan controlplane.ControlPlaneEvent, func()) {
	args := m.Called(ctx, id)
	ch := args.Get(0)
	if ch == nil {
		return nil, args.Get(1).(func())
	}
	return ch.(<-chan controlplane.ControlPlaneEvent), args.Get(1).(func())
}

func (m *mockControlPlane) SubscribeFiltered(ctx context.Context, filter controlplane.EventFilter) (<-chan controlplane.ControlPlaneEvent, func()) {
	args := m.Called(ctx, filter)
	ch := args.Get(0)
	if ch == nil {
		return nil, args.Get(1).(func())
	}
	return ch.(<-chan controlplane.ControlPlaneEvent), args.Get(1).(func())
}

func (m *mockControlPlane) GetHealthStatus(id controlplane.WorkflowID) (controlplane.HealthStatus, bool) {
	args := m.Called(id)
	return args.Get(0).(controlplane.HealthStatus), args.Bool(1)
}

func (m *mockControlPlane) Shutdown(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
