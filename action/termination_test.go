package action

import (
	"testing"

	"github.com/hortonworks/cloud-cost-reducer/context"
	"github.com/hortonworks/cloud-cost-reducer/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

func TestTerminationInit(t *testing.T) {
	assert.NotNil(t, context.Actions[types.TERMINATION_ACTION])
}

type mockProvider struct {
	calls int
}

func (p *mockProvider) GetRunningInstances() ([]*types.Instance, error) {
	return nil, nil
}

func (p *mockProvider) TerminateInstances([]*types.Instance) error {
	p.calls++
	return nil
}

func (p *mockProvider) GetAccesses() ([]*types.Access, error) {
	return nil, nil
}

type terminationSuite struct {
	suite.Suite
	providers    map[types.CloudType]func() types.CloudProvider
	mockProvider *mockProvider
}

func (s *terminationSuite) SetupSuite() {
	s.providers = context.CloudProviders
}

func (s *terminationSuite) SetupTest() {
	s.mockProvider = &mockProvider{0}
	context.CloudProviders = map[types.CloudType]func() types.CloudProvider{
		types.AWS: func() types.CloudProvider {
			return s.mockProvider
		}}
}

func (s *terminationSuite) TearDownSuite() {
	context.CloudProviders = s.providers
}

func (s *terminationSuite) TestTermination() {
	action := TerminationAction{}
	op := types.LONGRUNNING
	items := []types.CloudItem{
		types.Instance{CloudType: types.AWS},
		types.Instance{CloudType: types.GCP},
	}

	action.Execute(&op, items)

	s.Equal(1, s.mockProvider.calls)
}

func TestTerminationSuite(t *testing.T) {
	suite.Run(t, new(terminationSuite))
}
