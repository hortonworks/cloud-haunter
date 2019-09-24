package operation

import (
	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	"github.com/hortonworks/cloud-haunter/utils"
	log "github.com/sirupsen/logrus"
)

func filter(filterName string, items []types.CloudItem, filterType types.FilterConfigType, isNeeded func(types.CloudItem) bool) []types.CloudItem {
	var filtered []types.CloudItem
	for _, item := range items {
		var include bool
		if filterType.IsInclusive() {
			include = isFilterMatch(filterName, item, filterType, ctx.FilterConfig)
		} else {
			include = !isFilterMatch(filterName, item, filterType, ctx.FilterConfig)
		}
		if include {
			log.Debugf("[%s] item %s is not filtered, because of filter config", filterName, item.GetName())
		} else {
			log.Debugf("[%s] item %s is filtered, because of filter config", filterName, item.GetName())
		}
		if isNeeded(item) && include {
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

func isFilterMatch(filterName string, item types.CloudItem, filterType types.FilterConfigType, filterConfig *types.FilterConfig) bool {
	switch item.GetItem().(type) {
	case types.Instance:
		inst := item.GetItem().(types.Instance)
		name := item.GetName()
		ignoreLabelFound := utils.IsAnyMatch(inst.Tags, ctx.IgnoreLabel)
		if ignoreLabelFound {
			log.Debugf("[%s] Found ignore label on item: %s, label: %s", filterName, name, ctx.IgnoreLabel)
			if filterType.IsInclusive() {
				log.Debugf("[%s] inclusive filter applied on item: %s", filterName, name)
				return false
			}
			log.Debugf("[%s] exclusive filter applied on item: %s", filterName, name)
			return true
		}
		filtered, applied := applyFilterConfig(filterConfig, filterType, item, filterName, inst.Tags)
		if applied {
			return filtered
		}
	case types.Stack:
		stack := item.GetItem().(types.Stack)
		name := item.GetName()
		ignoreLabelFound := utils.IsAnyMatch(stack.Tags, ctx.IgnoreLabel)
		if ignoreLabelFound {
			log.Debugf("[%s] Found ignore label on item: %s, label: %s", filterName, name, ctx.IgnoreLabel)
			if filterType.IsInclusive() {
				log.Debugf("[%s] inclusive filter applied on item: %s", filterName, name)
				return false
			}
			log.Debugf("[%s] exclusive filter applied on item: %s", filterName, name)
			return true
		}
		filtered, applied := applyFilterConfig(filterConfig, filterType, item, filterName, stack.Tags)
		if applied {
			return filtered
		}
	case types.Access:
		accessFilter, _ := getFilterConfigs(filterConfig, filterType)
		if accessFilter != nil {
			switch item.GetCloudType() {
			case types.AWS:
				return isNameOrOwnerMatch(filterName, item, accessFilter.Aws.Names, accessFilter.Aws.Owners)
			case types.AZURE:
				return isNameOrOwnerMatch(filterName, item, accessFilter.Azure.Names, accessFilter.Azure.Owners)
			case types.GCP:
				return isNameOrOwnerMatch(filterName, item, accessFilter.Gcp.Names, accessFilter.Gcp.Owners)
			default:
				log.Warnf("[%s] Cloud type not supported: %s", filterName, item.GetCloudType())
			}
		}
	}
	return false
}

func applyFilterConfig(filterConfig *types.FilterConfig, filterType types.FilterConfigType, item types.CloudItem, filterName string, tags types.Tags) (applied, filtered bool) {
	_, instanceFilter := getFilterConfigs(filterConfig, filterType)
	if instanceFilter != nil {
		switch item.GetCloudType() {
		case types.AWS:
			return isMatchWithIgnores(filterName, item, tags,
				instanceFilter.Aws.Names, instanceFilter.Aws.Owners, instanceFilter.Aws.Labels), true
		case types.AZURE:
			return isMatchWithIgnores(filterName, item, tags,
				instanceFilter.Azure.Names, instanceFilter.Azure.Owners, instanceFilter.Azure.Labels), true
		case types.GCP:
			return isMatchWithIgnores(filterName, item, tags,
				instanceFilter.Gcp.Names, instanceFilter.Gcp.Owners, instanceFilter.Gcp.Labels), true
		default:
			log.Warnf("[%s] Cloud type not supported: %s", filterName, item.GetCloudType())
		}
	}
	return false, false
}

func getFilterConfigs(filterConfig *types.FilterConfig, filterType types.FilterConfigType) (accessConfig *types.FilterAccessConfig, instanceConfig *types.FilterInstanceConfig) {
	if filterConfig != nil {
		if filterType.IsInclusive() {
			return filterConfig.IncludeAccess, filterConfig.IncludeInstance
		}
		return filterConfig.ExcludeAccess, filterConfig.ExcludeInstance
	}
	return nil, nil
}

func isMatchWithIgnores(filterName string, item types.CloudItem, tags map[string]string, names, owners []string, labels []string) bool {
	if isNameOrOwnerMatch(filterName, item, names, owners) || utils.IsAnyStartsWith(tags, labels...) {
		log.Debugf("[%s] item %s match with name/owner or tag %s", filterName, item.GetName(), labels)
		return true
	}
	log.Debugf("[%s] item %s does not match with name/owner or tag %s", filterName, item.GetName(), labels)
	return false
}

func isNameOrOwnerMatch(filterName string, item types.CloudItem, names, owners []string) bool {
	if utils.IsStartsWith(item.GetName(), names...) || utils.IsStartsWith(item.GetOwner(), owners...) {
		log.Debugf("[%s] item %s match with filter config name %s or owner %s", filterName, item.GetName(), names, owners)
		return true
	}
	log.Debugf("[%s] item %s does not match with filter config name %s or owner %s", filterName, item.GetName(), names, owners)
	return false
}
