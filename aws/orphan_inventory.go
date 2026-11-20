package aws

import (
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/hortonworks/cloud-haunter/types"
)

// Immutable, lazy ownership snapshots live for one region and one operation.
// Selection and termination use distinct snapshots, never a process-wide cache.
type orphanLBInventory struct {
	region       string
	cfClient     cfClient
	ec2Client    ec2Client
	seeded       bool
	stacks       []*types.Stack
	cfOnce       sync.Once
	cfIDs        map[string]bool
	cfErr        error
	endpointOnce sync.Once
	endpointIDs  map[string]bool
	endpointErr  error
}

func (i *orphanLBInventory) cloudFormation() ([]*types.Stack, map[string]bool, error) {
	i.cfOnce.Do(func() {
		i.cfIDs = map[string]bool{}
		if !i.seeded {
			if i.cfClient == nil {
				i.cfErr = fmt.Errorf("missing CloudFormation client for orphan verification in %s", i.region)
				return
			}
			request := &cloudformation.DescribeStacksInput{}
			for {
				output, err := i.cfClient.DescribeStacks(request)
				if err != nil {
					i.cfErr = err
					return
				}
				for _, s := range output.Stacks {
					i.stacks = append(i.stacks, &types.Stack{ID: aws.StringValue(s.StackId), Region: i.region, Tags: getCFTags(s.Tags)})
				}
				if aws.StringValue(output.NextToken) == "" {
					break
				}
				request.SetNextToken(*output.NextToken)
			}
		}
		for _, s := range i.stacks {
			if s.Region != "" && s.Region != i.region {
				continue
			}
			if s.ID == "" || i.cfClient == nil {
				i.cfErr = fmt.Errorf("unverifiable CloudFormation identity/client in %s", i.region)
				return
			}
			request := &cloudformation.ListStackResourcesInput{StackName: aws.String(s.ID)}
			for {
				output, err := i.cfClient.ListStackResources(request)
				if err != nil {
					i.cfErr = err
					return
				}
				for _, r := range output.StackResourceSummaries {
					if id := aws.StringValue(r.PhysicalResourceId); id != "" {
						i.cfIDs[id] = true
					}
				}
				if aws.StringValue(output.NextToken) == "" {
					break
				}
				request.SetNextToken(*output.NextToken)
			}
		}
	})
	return i.stacks, i.cfIDs, i.cfErr
}

func (i *orphanLBInventory) endpointServices() (map[string]bool, error) {
	i.endpointOnce.Do(func() {
		i.endpointIDs = map[string]bool{}
		request := &ec2.DescribeVpcEndpointServiceConfigurationsInput{}
		for {
			output, err := i.ec2Client.DescribeVpcEndpointServiceConfigurations(request)
			if err != nil {
				i.endpointErr = err
				return
			}
			for _, service := range output.ServiceConfigurations {
				for _, arn := range service.NetworkLoadBalancerArns {
					i.endpointIDs[aws.StringValue(arn)] = true
				}
			}
			if aws.StringValue(output.NextToken) == "" {
				break
			}
			request.SetNextToken(*output.NextToken)
		}
	})
	return i.endpointIDs, i.endpointErr
}
