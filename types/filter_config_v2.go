package types

import (
	log "github.com/sirupsen/logrus"
	"strings"
)

// FilterConfigV2 structure that stores the information provided by the exclude/include flag
type FilterConfigV2 struct {
	Filters []FilterConfigV2Filter `yaml:"filters"`
}

type FilterConfigV2Filter struct {
	Types      []FilterEntityType     `yaml:"filterTypes"`
	CloudTypes []CloudType            `yaml:"cloudTypes"`
	Properties []FilterConfigProperty `yaml:"filterProperties"`
	Values     []string               `yaml:"filterValues"`
}

func (filterConfig FilterConfigV2) GetFilterValues(fType FilterEntityType, cloud CloudType, property FilterConfigProperty) []string {
	cloudProperty := strings.ToLower(string(cloud))
	log.Debugf("FilterEntityType: %s, CloudProperty: %s, FilterConfigProperty: %s", fType, cloudProperty, property)

	for _, filter := range filterConfig.Filters {
		for _, fcType := range filter.Types {
			if fcType == fType {
				for _, fcCloud := range filter.CloudTypes {
					if string(fcCloud) == cloudProperty {
						for _, fcProperty := range filter.Properties {
							if fcProperty == property {
								return filter.Values
							}
						}
					}
				}
			}
		}
	}
	return nil
}
