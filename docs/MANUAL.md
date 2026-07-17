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