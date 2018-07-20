package action

import (
	"testing"

	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

func xTestTerminationInit(t *testing.T) {
	assert.NotNil(t, ctx.Actions[types.TerminationAction])
}

type mockProvider struct {
	calls int
}

func (p *mockProvider) GetInstances() ([]*types.Instance, error) {
	return nil, nil
}

func (p *mockProvider) TerminateInstances([]*types.Instance) error {
	p.calls++
	return nil
}

func (p *mockProvider) StopInstances([]*types.Instance) error {
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
	s.providers = ctx.CloudProviders
}

func (s *terminationSuite) SetupTest() {
	s.mockProvider = &mockProvider{0}
	ctx.CloudProviders = map[types.CloudType]func() types.CloudProvider{
		types.AWS: func() types.CloudProvider {
			return s.mockProvider
		}}
}

func (s *terminationSuite) TearDownSuite() {
	ctx.CloudProviders = s.providers
}

func (s *terminationSuite) TestTermination() {
	action := terminationAction{}
	op := types.Instances
	items := []types.CloudItem{
		types.Instance{CloudType: types.AWS},
		types.Instance{CloudType: types.GCP},
	}

	action.Execute(op, []types.FilterType{}, items)

	s.Equal(1, s.mockProvider.calls)
}

func TestTerminationSuite(t *testing.T) {
	suite.Run(t, new(terminationSuite))
}
