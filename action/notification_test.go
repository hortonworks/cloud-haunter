package action

import (
	"errors"
	"sync"
	"testing"

	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
)

type mockDispatcher struct {
	mu    sync.Mutex
	calls int
	err   error
}

func (d *mockDispatcher) GetName() string {
	return "mock"
}

func (d *mockDispatcher) Send(op types.OpType, filters []types.FilterType, items []types.CloudItem) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls++
	return d.err
}

func TestNotification(t *testing.T) {
	t.Parallel()
	mock := &mockDispatcher{}
	cfg := &config.Config{Dispatchers: map[string]types.Dispatcher{"mock": mock}}

	err := NewNotification(cfg).Execute(types.Instances, []types.FilterType{}, []types.CloudItem{types.Access{}})

	assert.NoError(t, err)
	assert.Equal(t, 1, mock.calls)
}

// TestNotificationSkipsWhenNoItems covers the len(items) > 0 guard: no
// dispatcher is invoked when there is nothing to report.
func TestNotificationSkipsWhenNoItems(t *testing.T) {
	t.Parallel()
	mock := &mockDispatcher{}
	cfg := &config.Config{Dispatchers: map[string]types.Dispatcher{"mock": mock}}

	err := notificationAction{cfg}.Execute(types.Instances, nil, nil)

	assert.NoError(t, err)
	assert.Equal(t, 0, mock.calls)
}

// TestNotificationFansOutToAllDispatchers verifies every registered dispatcher
// receives the notification.
func TestNotificationFansOutToAllDispatchers(t *testing.T) {
	t.Parallel()
	a := &mockDispatcher{}
	b := &mockDispatcher{}
	cfg := &config.Config{Dispatchers: map[string]types.Dispatcher{"a": a, "b": b}}

	err := notificationAction{cfg}.Execute(types.Instances, nil, []types.CloudItem{types.Access{}})

	assert.NoError(t, err)
	assert.Equal(t, 1, a.calls)
	assert.Equal(t, 1, b.calls)
}

// TestNotificationToleratesDispatcherError verifies a dispatcher error is logged
// rather than propagated (Execute must not panic).
func TestNotificationToleratesDispatcherError(t *testing.T) {
	t.Parallel()
	mock := &mockDispatcher{err: errors.New("boom")}
	cfg := &config.Config{Dispatchers: map[string]types.Dispatcher{"mock": mock}}

	err := notificationAction{cfg}.Execute(types.Instances, nil, []types.CloudItem{types.Access{}})

	assert.NoError(t, err)
	assert.Equal(t, 1, mock.calls)
}
