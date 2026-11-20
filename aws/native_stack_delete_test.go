package aws

import (
	"errors"
	"testing"

	elb "github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
)

type failingDeleteElbClient struct {
	mockElbClient
	deleteErrors map[string]error
}

func (t failingDeleteElbClient) DeleteLoadBalancer(input *elb.DeleteLoadBalancerInput) (*elb.DeleteLoadBalancerOutput, error) {
	arn := *input.LoadBalancerArn
	t.operationChannel <- "DeleteLoadBalancer:" + arn
	return nil, t.deleteErrors[arn]
}

func closeAndCollectOperations(operationChannel chan string) []string {
	close(operationChannel)

	operations := []string{}
	for operation := range operationChannel {
		operations = append(operations, operation)
	}
	return operations
}

func TestDeleteNativeStackLoadBalancersOnly(t *testing.T) {
	operationChannel := make(chan string, 16)
	stack := &types.Stack{
		ID:   "stack-id",
		Name: "stack",
		Metadata: map[string]string{
			METADATA_TYPE:           TYPE_NATIVE,
			METADATA_LOAD_BALANCERS: "lb-1,lb-2",
		},
	}

	err := deleteNativeStack(
		mockEc2Client{operationChannel: operationChannel},
		mockElbClient{operationChannel: operationChannel},
		mockCwClient{operationChannel: operationChannel},
		"eu-central-1",
		stack,
	)

	assert.Nil(t, err)
	assert.ElementsMatch(t, []string{
		"ModifyLoadBalancerAttributes:lb-1",
		"DeleteLoadBalancer:lb-1",
		"ModifyLoadBalancerAttributes:lb-2",
		"DeleteLoadBalancer:lb-2",
	}, closeAndCollectOperations(operationChannel))
}

func TestDeleteNativeStackWithoutResourcesMakesNoCalls(t *testing.T) {
	operationChannel := make(chan string, 16)
	stack := &types.Stack{
		ID:       "stack-id",
		Name:     "stack",
		Metadata: map[string]string{},
	}

	err := deleteNativeStack(
		mockEc2Client{operationChannel: operationChannel},
		mockElbClient{operationChannel: operationChannel},
		mockCwClient{operationChannel: operationChannel},
		"eu-central-1",
		stack,
	)

	assert.Nil(t, err)
	assert.Empty(t, closeAndCollectOperations(operationChannel))
}

func TestDeleteNativeStackJoinsConcurrentErrors(t *testing.T) {
	operationChannel := make(chan string, 16)
	errLb1 := errors.New("lb-1 delete failed")
	errLb2 := errors.New("lb-2 delete failed")
	failingElb := failingDeleteElbClient{
		mockElbClient: mockElbClient{operationChannel: operationChannel},
		deleteErrors: map[string]error{
			"lb-1": errLb1,
			"lb-2": errLb2,
		},
	}
	stack := &types.Stack{
		ID:   "stack-id",
		Name: "stack",
		Metadata: map[string]string{
			METADATA_TYPE:           TYPE_NATIVE,
			METADATA_LOAD_BALANCERS: "lb-1,lb-2",
		},
	}

	err := deleteNativeStack(
		mockEc2Client{operationChannel: operationChannel},
		failingElb,
		mockCwClient{operationChannel: operationChannel},
		"eu-central-1",
		stack,
	)

	assert.ErrorIs(t, err, errLb1)
	assert.ErrorIs(t, err, errLb2)
	assert.ElementsMatch(t, []string{
		"ModifyLoadBalancerAttributes:lb-1",
		"DeleteLoadBalancer:lb-1",
		"ModifyLoadBalancerAttributes:lb-2",
		"DeleteLoadBalancer:lb-2",
	}, closeAndCollectOperations(operationChannel))
}
