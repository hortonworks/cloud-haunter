# Changelog
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
