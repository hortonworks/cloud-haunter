# Cloud Haunter Manual

This is the documentation for CH describing it's core functionality.
If you are a Cloudera employee for company specific documentation please see our internal wiki.

## Overview

Cloud Haunter supports the three major cloud providers: Azure, Google and Amazon.
It's primary job is to clean up resources to prevent overcharges and large costs associated with forgotten resources.

A typical run of consists of three parts: an operation, a filter and an action.
An operation collects specific resources from one or more cloud providers then they are optionally filtered and finally the selected action is performed on each of the collected resources.
The filter is technically optional, multiple filters can be specified, and when present it accepts a YAML config file.

This tool works well if you use it from early days of your cloud account and all of your users are following the basic rules of tagging/labeling your cloud resources. On the other hand introducing it in an existing environment can problematic.

## Usage

Generally an operation (`-o`), a filter (`-f` and `-fc`) and an action (`-a`) to perform needs to be specified. Furthermore at least one cloud provider needs to be specified (`-c`).

The appropriate [environment variables](#environment-variables) for your cloud provider also needs to be present (API/Access keys, etc).

Here is a more complete example:
```
./ch -a termination -o getInstances -c gcp -f nomatch -fc cloud-haunter-config/filter-configs/known-owners-filter-config-v2.yml
```

Running the above would result in CH retrieving all VM instances on GCP and terminate those that do not belong to any of the known owners in the specified filter config.

So the general steps are:
1. [Set up the environment variables.](#environment-variables)
2. Create the filter config file. [Detailed description here.](FILTER_CONFIG_V2.md)
3. Call cloud haunter with the appropriate flags. See [here](DETAILS_OFA.md) for a detailed description of operations, filters and actions.

*Note: Using a filter is recommended unless you want to clean up everything or you are generating a report.*

*Note: It's also recommended to use either the dry run option or the `report-cloud-items` action to verify the filter configuration.*

*Note: Unless you merely want to filter using the names of resources appropriate labels/tags need to be setup on your side for each resource. Such as the `owner` label.*

See below for the [complete description of cli options](#cli-options).

## Environment variables
### Cloud specific
| Cloud provider | Environment Variable | Description / Default |
| --- | --- | --- |
| **AWS** | `AWS_ACCESS_KEY_ID` | — |
| **AWS** | `AWS_SECRET_ACCESS_KEY` | — |
| **Azure** | `AZURE_SUBSCRIPTION_ID` | — |
| **Azure** | `AZURE_TENANT_ID` | — |
| **Azure** | `AZURE_CLIENT_ID` | — |
| **Azure** | `AZURE_CLIENT_SECRET` | — |
| **Google** | `GOOGLE_PROJECT_ID` | — |
| **Google** | `GOOGLE_APPLICATION_CREDENTIALS` | Location of service account JSON |
| **Google** | `GCP_BILLING_EXPORT_TABLE` | Billing export table as `project.dataset.table`, only needed by `budget-notification` |
### Others
| - | Environment Variable | Description / Default |
| --- | --- | --- |
| **HipChat** | `HIPCHAT_TOKEN` | — |
| **HipChat** | `HIPCHAT_SERVER` | — |
| **HipChat** | `HIPCHAT_ROOM` | — |
| **Slack** | `SLACK_WEBHOOK_URL` | — |
| **Long running** | `RUNNING_PERIOD` | Default: `24h` |
| **Old access** | `ACCESS_AVAILABLE_PERIOD` | Default: `2880h` |
| **Retention days for cleanup** | `RETENTION_DAYS` | Default: `90` |
| **Slack** | `SLACK_OWNER_IDS_FILE` | YAML map of owner label values to Slack member IDs, used to @-mention owners in budget reminders |
| **Budget reminder** | `BUDGET_PERIOD` | How far back owner spend is summed, rounded up to whole UTC days before today. Default: `168h`, at least `24h` |
| **Budget reminder** | `BUDGET_LIMIT` | Owner spend in USD above which the owner is reminded. Must be finite and non-negative; `0` reminds for any positive spend. Default: `100` |

## Budget reminder

The `budget-notification` action reminds owners who keep ignore-labelled resources running for too long:
an owner is reminded when their ignore-labelled spend over `BUDGET_PERIOD` exceeds `BUDGET_LIMIT`
and at least one of their ignore-labelled cloud items reaches the action. The spend is summed across every selected cloud
over the same whole UTC days, ending yesterday, with owners normalized (lower case, e-mail domain removed). Every account must
bill in USD, because the amounts are added up without currency conversion. Owners on clouds that cannot report costs are not reminded.
When a cloud fails to report its costs, the owners of the other clouds are still reminded, then the run fails.

A daily run that reminds about resources running for more than a week:
```
RUNNING_PERIOD=168h BUDGET_PERIOD=168h BUDGET_LIMIT=100 \
./ch -o getBillableResources -c AWS,AZURE,GCP -f ignored,longrunning,nomatch -fc exceptions.yml -i -a budget-notification
```
`-i` is required, otherwise `longrunning` and `nomatch` drop the ignore-labelled items. Owners listed under `excludeInstance` in the
filter config are never reminded, which exempts infrastructure owners. The action applies that list itself too, so it also covers the
Azure items that only inherit their owner from the resource group.

Where the spend comes from, and the read permissions it needs:

| Cloud | Source | Permission | Per-resource cost |
| --- | --- | --- | --- |
| **AWS** | Cost Explorer, grouped by the ignore and owner tags (both must be activated as cost allocation tags) | `ce:GetCostAndUsage`, `ce:GetCostAndUsageWithResources` | EC2 instances only, over `BUDGET_PERIOD` capped at the API's 14-day resource-level history limit; needs resource-level data enabled in Cost Explorer |
| **Azure** | Cost Management query over the ignore-labelled resource groups and resources, by the owner tag or else the resource group's owner tag | Cost Management Reader on the subscription | Yes |
| **Google** | Cloud Billing export to BigQuery (`GCP_BILLING_EXPORT_TABLE`) | BigQuery Job User on the project, Data Viewer on the export dataset | No |

AWS owner totals always cover the full `BUDGET_PERIOD`, even when per-instance costs are capped at 14 days.
Azure cost queries follow all result pages and sum their costs; a failed page fails the query rather than returning partial totals.

Cost Explorer charges for each request. GovCloud is not supported. On AWS the ignore tag needs a value for its spend to count,
because Cost Explorer reports a tag with an empty value as a missing tag. The action warns about every such item that reaches it.

## CLI Options

| Option/switch | Description | 
|----------|------------------------------------------------------|
| -h       | Prints help                                          |
| -o       | Operation to perform                                 |
| -f       | Comma separated list of filters to apply             |
| -a       | Action to apply to the filtered cloud items          |
| -c       | Comma separated list of clud providers               |
| -fc      | Path to the filter config (YAML)                     |
| -d       | Dry-run: action won't be executed                    |
| -v       | Verbose logging                                      |
| -i       | Don't respect the ignore label                       |
| -e       | For the owner tag exact matching is used instead of partial matches. |
| -excludeAwsRegion | Region exclude for AWS only. Comma separated list of regions. |