package operation

import (
	log "github.com/Sirupsen/logrus"
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/hortonworks/cloud-haunter/utils"
)

func filter(items []types.CloudItem, isNeeded func(types.CloudItem) bool) []types.CloudItem {
	var filtered []types.CloudItem
	for _, item := range items {
		if isNeeded(item) && !isIgnored(item, ctx.Ignores) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func isInstance(item types.CloudItem) bool {
	switch item.GetItem().(type) {
	case types.Instance:
		return true
	default:
		return false
	}
}

func isIgnored(item types.CloudItem, ignores *types.Ignores) bool {
	switch item.GetItem().(type) {
	case types.Instance:
		inst := item.GetItem().(types.Instance)
		if utils.IsAnyMatch(inst.Tags, ctx.IgnoreLabels[item.GetCloudType()]) {
			return true
		}
		if ignores != nil {
			switch item.GetCloudType() {
			case types.AWS:
				return isMatchWithIgnores(item, inst.Tags,
					ignores.Instance.Aws.Names, ignores.Instance.Aws.Owners, ignores.Instance.Aws.Labels)
			case types.AZURE:
				return isMatchWithIgnores(item, inst.Tags,
					ignores.Instance.Azure.Names, ignores.Instance.Azure.Owners, ignores.Instance.Azure.Labels)
			case types.GCP:
				return isMatchWithIgnores(item, inst.Tags,
					ignores.Instance.Gcp.Names, ignores.Instance.Gcp.Owners, ignores.Instance.Gcp.Labels)
			default:
				log.Warnf("[FILTER] Cloud type not supported: ", item.GetCloudType())
			}
		}
	case types.Access:
		if ignores != nil {
			switch item.GetCloudType() {
			case types.AWS:
				return isNameOrOwnerMatch(item, ignores.Access.Aws.Names, ignores.Access.Aws.Owners)
			case types.AZURE:
				return isNameOrOwnerMatch(item, ignores.Access.Azure.Names, ignores.Access.Azure.Owners)
			case types.GCP:
				return isNameOrOwnerMatch(item, ignores.Access.Gcp.Names, ignores.Access.Gcp.Owners)
			default:
				log.Warnf("[FILTER] Cloud type not supported: ", item.GetCloudType())
			}
		}
	}
	return false
}

func isMatchWithIgnores(item types.CloudItem, tags map[string]string, names, owners []string, labels []string) bool {
	if isNameOrOwnerMatch(item, names, owners) || utils.IsAnyStartsWith(tags, labels...) {
		return true
	}
	return false
}

func isNameOrOwnerMatch(item types.CloudItem, names, owners []string) bool {
	return utils.IsStartsWith(item.GetName(), names...) || utils.IsStartsWith(item.GetOwner(), owners...)
}
