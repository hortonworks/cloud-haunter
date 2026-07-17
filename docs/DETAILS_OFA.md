# Detailed overview

Note: internally the result of every operation is abstracted away as **Cloud Item**. This term refers to any cloud resource, regardless of its specific type or origin (which provider it came from). Consequently, both filters and actions operate uniformly on these generic Cloud Items.
As such this term is often used here and in other documents.

## Operations
Operations retrieve various cloud resources (in the general sense of the word).

| Operation | Description |
| --- | --- |
| **getInstances** | Retrieves VM instances. |
| **getAccess** | Retrieves users/accounts. Currently not supported on Azure. |
| **getDatabases** | Retrieves database instances. |
| **getDisks** | Retrieves disks. |
| **getImages** | Retrieves images. |
| **getStacks** | Retrieves CloudFormation stacks, resources. |
| **getAlerts** | Retrieves alerts on AWS. |
| **getStorages** | Retrieves S3 storage on AWS and functionally equivalent object storage on other cloud providers. |
| **getResources** | Retrieves load balancers, EC2 instances and VPCs. |

## Filters
A filter operates on the result of a operation.
There are essentially two types of filters: inclusive and excluding.
An inclusive filter will only match cloud items that are specified in it's configuration while an excluding (exclusive) filter will remove matching entries, but leave the rest.
Filters often use the values from the configuration file as a prefix when matching instead of requiring an exact match.
Regarding details about configuring a filter using the config YAML (see here)[FILTER_CONFIG_V2.md].

| Filter | Description |
| --- | --- |
| **longrunning** | Filters the cloud items that are created/running after a certain time. Affected by the `RUNNING_PERIOD` envvar. |
| **ownerless** | Filters the cloud items that do not have the 'Owner' tag. |
| **oldaccess** | Filters the cloud access objects that are created a long time ago.  Affected by the `ACCESS_AVAILABLE_PERIOD` envvar.|
| **stopped** | Filters the cloud items whose state is stopped. |
| **running** | Filters the cloud items whose state is running. |
| **failed** | Filters the cloud items whose state is failed. |
| **unused** | Filters the items that are not used. Only works on disks and alerts. |
| **match** | Filters the items that match the include criteria of the filter config. |
| **nomatch** | Filters the items that do not match the include criteria of the filter config. |

## Actions

Actions are executed on the filtered list of Cloud Items retrieved by the operation specified.

*Note: Using a filter is recommended unless you want to clean up everything or you are generating a report.*

*Note: It's also recommended to use either the dry run option or the `report-cloud-items` action to verify the filter configuration.*


| Action | Description |
| --- | --- |
| **log** | Logs the cloud items to the console. |
| **json** | Prints the output in processable JSON format. |
| **stop** | Stops the cloud item if the item itself supports such operation. |
| **notification** | Sends a notification through the dispatcher interface. |
| **termination** | Terminates the cloud item if the item supports such operation. |
| **cleanup** | Cleans up the cloud item if the item supports such operation. |
| **report-cloud-items** | Generates a CSV report printing information about all the cloud items present as input to this action. Useful to test what The Haunter sees. Essentially similar to log and json. It's also WIP. |