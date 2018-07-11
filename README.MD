# Cloud Cost Reducer

As the name suggests the main goal of this project is to save costs in the cloud. Cloud Cost Reducer (ccr) is supports the three major cloud providers.

`The project is under construction, use on your own risk!`

Current version of the software has limited features, it can monitor and report (only HipChat supported)
 * long running instances
 * instances without owner
 * old credentials (AWS only).

For the proper work, you have to use some custom tags/labels on your cloud resources.

#### AWS
 * Owner tag: `Owner`
 * Ignore tag: `cloud-cost-reducer-ignore`

#### Azure
 * Owner tag: `Owner`
 * Ignore tag: `cloud-cost-reducer-ignore`
 * Resource creation time, because Azure SDK doesn't response it: `cb-creation-timestamp`

#### Google
 * Owner label: `owner`
 * Ignore label: `cloud-cost-reducer-ignore`

## Future plans
 * automatic instance termination
 * more notification types
 * looking for orphan resources (images, volumes, IPs)
 * multiple cloud accounts

```
# ccr -h
NAME:
   Cloud Cost Reducer
USAGE:
   ccr -o=operation -a=action [-c=cloud]

VERSION:

AUTHOR(S):
   Hortonworks

OPERATIONS:
        -o longrunning
        -o oldaccess
        -o ownerless
ACTIONS:
        -a log
        -a notification
CLOUDS:
        -c AWS
        -c AZURE
        -c GCP
DRY RUN:
        -d
VERBOSE:
        -v
HELP:
        -p
```
