package action

import (
	"testing"

	ctx "github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

func TestNotificationInit(t *testing.T) {
	assert.NotNil(t, ctx.Actions[types.NOTIFICATION_ACTION])
}

type mockDispatcher struct {
	calls int
}

func (d *mockDispatcher) GetName() string {
	return "mock"
}

func (d *mockDispatcher) Send(op *types.OpType, items []types.CloudItem) error {
	d.calls++
	return nil
}

type notificationSuite struct {
	suite.Suite
	dispatchers    map[string]types.Dispatcher
	mockDispatcher *mockDispatcher
}

func (s *notificationSuite) SetupSuite() {
	s.dispatchers = ctx.Dispatchers
}

func (s *notificationSuite) SetupTest() {
	s.mockDispatcher = &mockDispatcher{0}
	ctx.Dispatchers = map[string]types.Dispatcher{"mock": s.mockDispatcher}
}

func (s *notificationSuite) TearDownSuite() {
	ctx.Dispatchers = s.dispatchers
}

func (s *notificationSuite) TestNotification() {
	action := notificationAction{}
	op := types.LONGRUNNING
	items := []types.CloudItem{types.Access{}}

	action.Execute(&op, items)

	s.Equal(1, s.mockDispatcher.calls)
}

func TestNotificationSuite(t *testing.T) {
	suite.Run(t, new(notificationSuite))
}
