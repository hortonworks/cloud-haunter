package aws

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	elb "github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/hortonworks/cloud-haunter/config"
	"github.com/hortonworks/cloud-haunter/types"
	log "github.com/sirupsen/logrus"
)

const cloudbreakResourceTag = "Cloudera-Resource-Name"

func isMissingOrphanResource(err error) bool {
	if e, ok := err.(awserr.Error); ok {
		return e.Code() == "LoadBalancerNotFound" || e.Code() == "TargetGroupNotFound"
	}
	return false
}

func isTerminatedInstance(i *ec2.Instance) bool {
	return i.State != nil && (aws.StringValue(i.State.Name) == ec2.InstanceStateNameTerminated || aws.Int64Value(i.State.Code)&0xff == 48)
}

// An active LB is not necessarily in use. Only tagged, consistently owned NLBs
// with no compute, CF ownership, registered targets or endpoint service qualify.
// No EIPs, security groups or target groups are attached to this synthetic stack.
func assembleOrphanLoadBalancerStack(cfg *config.Config, region, group string, lbs []*elb.LoadBalancer, tagsByARN map[string]types.Tags, instances []*ec2.Instance, cfStacks []*types.Stack, ec2Client ec2Client, elbClient elbClient, inventories ...*orphanLBInventory) (*AwsNativeStack, error) {
	if group == "" || len(lbs) == 0 {
		return nil, nil
	}
	for _, s := range cfStacks {
		if (s.Region == region || s.Region == "") && s.Tags[cfg.ResourceGroupingLabel] == group {
			return nil, nil
		}
	}
	stack := &AwsNativeStack{Type: TYPE_NATIVE_LB, ID: group, Region: region, State: types.Unused, Tags: types.Tags{}}
	resourceCRNs := map[string]bool{}
	for n, lb := range lbs {
		arn := aws.StringValue(lb.LoadBalancerArn)
		tags := tagsByARN[arn]
		if arn == "" || aws.StringValue(lb.Type) != elb.LoadBalancerTypeEnumNetwork || lb.CreatedTime == nil || lb.CreatedTime.IsZero() || tags[cfg.ResourceGroupingLabel] != group || tags[cfg.OwnerLabel] == "" {
			return nil, nil
		}
		if _, cfOwned := tags[CF_TAG]; cfOwned {
			return nil, nil
		}
		// Provisioning LBs may not have their instances/targets yet.
		if lb.State == nil || aws.StringValue(lb.State.Code) != elb.LoadBalancerStateEnumActive {
			return nil, nil
		}
		if n == 0 {
			stack.Owner = tags[cfg.OwnerLabel]
			for k, v := range tags {
				stack.Tags[k] = v
			}
		} else {
			if tags[cfg.OwnerLabel] != stack.Owner {
				return nil, nil
			}
			// Only shared tag values should participate in group-level matching.
			for k, v := range stack.Tags {
				if value, present := tags[k]; k != cfg.IgnoreLabel && (!present || value != v) {
					delete(stack.Tags, k)
				}
			}
		}
		// Any member's ignore label protects the entire group, even if values differ.
		if v, ignored := tags[cfg.IgnoreLabel]; ignored {
			stack.Tags[cfg.IgnoreLabel] = v
		}
		if crn := tags[cloudbreakResourceTag]; crn != "" {
			resourceCRNs[crn] = true
		}
		stack.LoadBalancers = append(stack.LoadBalancers, arn)
		// Use the newest member: an old LB must not age a newly created LB into cleanup.
		if lb.CreatedTime.After(stack.Created) {
			stack.Created = *lb.CreatedTime
		}
	}
	for _, i := range instances {
		if isTerminatedInstance(i) {
			continue
		}
		tags := getEc2Tags(i.Tags)
		// Includes stopped, stopping, shutting-down and CF-managed instances.
		if tags[cfg.ResourceGroupingLabel] == group || resourceCRNs[tags[cloudbreakResourceTag]] {
			return nil, nil
		}
	}
	inventory := &orphanLBInventory{region: region, ec2Client: ec2Client, seeded: true, stacks: cfStacks}
	if len(inventories) > 0 {
		inventory = inventories[0]
	}
	currentCFStacks, cfIDs, err := inventory.cloudFormation()
	if err != nil {
		return nil, err
	}
	for _, s := range currentCFStacks {
		if (s.Region == region || s.Region == "") && s.Tags[cfg.ResourceGroupingLabel] == group {
			return nil, nil
		}
	}
	endpointIDs, err := inventory.endpointServices()
	if err != nil {
		return nil, err
	}
	for _, lb := range lbs {
		arn := aws.StringValue(lb.LoadBalancerArn)
		if cfIDs[arn] || endpointIDs[arn] {
			return nil, nil
		}
	}
	for _, lb := range lbs {
		request := &elb.DescribeTargetGroupsInput{LoadBalancerArn: lb.LoadBalancerArn}
		for {
			output, err := elbClient.DescribeTargetGroups(request)
			if err != nil {
				if isMissingOrphanResource(err) {
					return nil, nil
				}
				return nil, err
			}
			for _, tg := range output.TargetGroups {
				targets, err := elbClient.DescribeTargetHealth(&elb.DescribeTargetHealthInput{TargetGroupArn: tg.TargetGroupArn})
				if err != nil {
					if isMissingOrphanResource(err) {
						return nil, nil
					}
					return nil, err
				}
				// Even unhealthy/draining targets mean the LB is still referenced.
				if len(targets.TargetHealthDescriptions) != 0 {
					return nil, nil
				}
			}
			if aws.StringValue(output.NextMarker) == "" {
				break
			}
			request.SetMarker(*output.NextMarker)
		}
	}
	return stack, nil
}

// Refresh read-only eligibility at the action boundary, including dry-run.
func deleteOrphanLoadBalancerStack(cfg *config.Config, cfClient cfClient, ec2Client ec2Client, elbClient elbClient, cwClient cloudWatchClient, region string, stack *types.Stack, inventories ...*orphanLBInventory) error {
	refuse := func(reason string) error {
		log.Warnf("[AWS] Skipping orphan LB cleanup for %s in %s: %s", stack.ID, region, reason)
		return nil
	}
	for _, key := range []string{METADATA_INSTANCES, METADATA_VOLUMES, METADATA_SECURITY_GROUPS, METADATA_ELASTIC_IPS, METADATA_ALARMS} {
		if len(getResourceList(stack.Metadata[key])) != 0 {
			return fmt.Errorf("[AWS] Invalid orphan LB metadata for %s: contains non-LB resources", stack.ID)
		}
	}
	if cfg.ResourceGroupingLabel == "" || cfg.OwnerLabel == "" || stack.Owner == "" {
		return fmt.Errorf("[AWS] Missing grouping/owner identity for orphan LB stack %s", stack.ID)
	}
	var lbs []*elb.LoadBalancer
	tagsByARN := map[string]types.Tags{}
	resourceCRNs := map[string]bool{}
	for _, arn := range getResourceList(stack.Metadata[METADATA_LOAD_BALANCERS]) {
		output, err := elbClient.DescribeLoadBalancers(&elb.DescribeLoadBalancersInput{LoadBalancerArns: aws.StringSlice([]string{arn})})
		if err != nil {
			if e, ok := err.(awserr.Error); ok && e.Code() == "LoadBalancerNotFound" {
				continue
			}
			return err
		}
		if len(output.LoadBalancers) != 1 || aws.StringValue(output.LoadBalancers[0].LoadBalancerArn) != arn {
			return refuse("load-balancer identity could not be verified")
		}
		lb := output.LoadBalancers[0]
		tagsOutput, err := elbClient.DescribeTags(&elb.DescribeTagsInput{ResourceArns: aws.StringSlice([]string{arn})})
		if err != nil {
			if isMissingOrphanResource(err) {
				return refuse("load balancer disappeared during tag verification")
			}
			return err
		}
		for _, d := range tagsOutput.TagDescriptions {
			if aws.StringValue(d.ResourceArn) == arn {
				tagsByARN[arn] = getElbTags(d.Tags)
			}
		}
		tags := tagsByARN[arn]
		if tags[cfg.OwnerLabel] != stack.Owner || tags[cfg.ResourceGroupingLabel] != stack.ID {
			return refuse("ownership/group tags changed")
		}
		if _, ignored := tags[cfg.IgnoreLabel]; ignored && !cfg.IgnoreLabelDisabled {
			return refuse("ignore label added")
		}
		if crn := tags[cloudbreakResourceTag]; crn != "" {
			resourceCRNs[crn] = true
		}
		lbs = append(lbs, lb)
	}
	if len(lbs) == 0 {
		return nil
	} // Already deleted; no other cloud calls required.
	var instances []*ec2.Instance
	filters := []*ec2.Filter{{Name: aws.String("tag:" + cfg.ResourceGroupingLabel), Values: aws.StringSlice([]string{stack.ID})}}
	// Group and resource identities are ORed via separate requests, not ANDed.
	if len(resourceCRNs) > 0 {
		values := []string{}
		for crn := range resourceCRNs {
			values = append(values, crn)
		}
		filters = append(filters, &ec2.Filter{Name: aws.String("tag:" + cloudbreakResourceTag), Values: aws.StringSlice(values)})
	}
	for _, filter := range filters {
		request := &ec2.DescribeInstancesInput{Filters: []*ec2.Filter{filter}}
		for {
			output, err := ec2Client.DescribeInstances(request)
			if err != nil {
				return err
			}
			for _, reservation := range output.Reservations {
				instances = append(instances, reservation.Instances...)
			}
			if aws.StringValue(output.NextToken) == "" {
				break
			}
			request.SetNextToken(*output.NextToken)
		}
	}
	inventory := &orphanLBInventory{region: region, cfClient: cfClient, ec2Client: ec2Client}
	if len(inventories) > 0 {
		inventory = inventories[0]
	}
	checked, err := assembleOrphanLoadBalancerStack(cfg, region, stack.ID, lbs, tagsByARN, instances, nil, ec2Client, elbClient, inventory)
	if err != nil {
		return err
	}
	if checked == nil {
		return refuse("group has backing resources, targets, CF/endpoint ownership or unverifiable tags/type/state")
	}
	if !checked.Created.Add(cfg.LongRunningPeriod).Before(time.Now()) {
		return refuse("newest load balancer is inside the configured grace period")
	}
	if cfg.DryRun {
		log.Infof("[AWS] Dry-run revalidated %d orphan load balancers in group %s; no resources changed", len(checked.LoadBalancers), stack.ID)
		return nil
	}
	// Construct a new LB-only object instead of forwarding arbitrary old metadata.
	safe := &types.Stack{ID: stack.ID, Name: stack.Name, Region: region, Metadata: map[string]string{METADATA_LOAD_BALANCERS: strings.Join(checked.LoadBalancers, ",")}}
	log.Infof("[AWS] Revalidated %d orphan load balancers in group %s", len(checked.LoadBalancers), stack.ID)
	return deleteNativeStack(ec2Client, elbClient, cwClient, region, safe)
}
