# Changelog
## 0.7.1
- Dry run (`-d`) no longer changes resources when stopping or terminating: AWS and AWS GOV instance termination and stop (including auto scaling group suspension and spot instance termination) and RDS stop, Azure VM stop
- GCP instance termination verifies managed instance group membership (zonal and regional groups) and removes only the selected members from a group, which reduces the group's target size. A group is no longer deleted, not even when all of its members are selected, because it could have created an unselected member in the meantime: a fully selected group is kept with no members, an autoscaled group can scale up again. Termination waits until GCP has processed each deletion request (an instance deletion is finished, a group member deletion is scheduled) and reports failures instead of ignoring them, so the run exits with an error. Requires the `compute.instanceGroupManagers.get`, `compute.instanceGroupManagers.update`, `compute.zoneOperations.get` and `compute.regionOperations.get` permissions
- Unknown filter names (`-f`) fail the run before any cloud call, listing every unknown name. Filter names are case-insensitive
- Every name and `regexp:` alternative of a filter config name list is evaluated, not only the first one
- AWS S3 bucket tags and owner are read, so the ignore label and the owner filters apply to buckets. Requires the `s3:GetBucketTagging` permission. Buckets in excluded regions are left out
- AWS S3 cleanup skips a bucket whose objects cannot be listed instead of crashing
- An operation fails when a cloud provider returns an error while collecting its resources, so the run exits with an error before the action instead of running it without that provider's resources. Errors that a provider only logs (e.g. for a single AWS region) still give partial results. A cloud that does not support the operation (e.g. Azure disks) is skipped with a warning, it does not fail the run
## 0.6.1
- Added the `budget-notification` action, the `getBillableResources` operation and the `ignored` filter to remind owners about ignore-labelled resources that keep costing money
- Azure cloud items in an ignore-labelled resource group carry the group's ignore label and owner in their metadata
- Slack webhook errors are reported instead of being dropped
## 0.6.0
- Updated Azure client libs
- Updated GCP client libs
- Refactored global state
- Updated tests (Claude)
- Removed hipchat integration
