package types

import (
	log "github.com/sirupsen/logrus"
	"reflect"
	"strings"
)

type IFilterConfig interface {
	GetFilterValues(fType FilterEntityType, cloud CloudType, property FilterConfigProperty) []string
}

type FilterEntityType string

const (
	ExcludeAccess   = FilterEntityType("excludeAccess")
	IncludeAccess   = FilterEntityType("includeAccess")
	ExcludeInstance = FilterEntityType("excludeInstance")
	IncludeInstance = FilterEntityType("includeInstance")
)

type FilterConfigProperty string

const (
	Name  = FilterConfigProperty("name")
	Owner = FilterConfigProperty("owner")
	Label = FilterConfigProperty("label")
)

// FilterConfig structure that stores the information provided by the exclude/include flag
type FilterConfig struct {
	ExcludeAccess   *FilterAccessConfig   `yaml:"excludeAccess"`
	IncludeAccess   *FilterAccessConfig   `yaml:"includeAccess"`
	ExcludeInstance *FilterInstanceConfig `yaml:"excludeInstance"`
	IncludeInstance *FilterInstanceConfig `yaml:"includeInstance"`
}

// FilterAccessConfig filter properties for access items
type FilterAccessConfig struct {
	Aws struct {
		Names  []string `yaml:"names"`
		Owners []string `yaml:"owners"`
	} `yaml:"aws"`
	Azure struct {
		Names  []string `yaml:"names"`
		Owners []string `yaml:"owners"`
	} `yaml:"azure"`
	Gcp struct {
		Names  []string `yaml:"names"`
		Owners []string `yaml:"owners"`
	} `yaml:"gcp"`
}

// FilterInstanceConfig filter properties for instances
type FilterInstanceConfig struct {
	Aws struct {
		Labels []string `yaml:"labels"`
		Names  []string `yaml:"names"`
		Owners []string `yaml:"owners"`
	} `yaml:"aws"`
	Azure struct {
		Labels []string `yaml:"labels"`
		Names  []string `yaml:"names"`
		Owners []string `yaml:"owners"`
	} `yaml:"azure"`
	Gcp struct {
		Labels []string `yaml:"labels"`
		Names  []string `yaml:"names"`
		Owners []string `yaml:"owners"`
	} `yaml:"gcp"`
}

func (filterConfig FilterConfig) GetFilterValues(fType FilterEntityType, cloud CloudType, property FilterConfigProperty) []string {
	log.Debugf("fType: %s, cloud: %s, property :%s", fType, cloud, property)
	typeProperty := strings.ToUpper(string(string(fType)[0])) + string(fType)[1:]
	cloudProperty := string(string(cloud)[0]) + strings.ToLower(string(cloud)[1:])
	propertyProperty := strings.ToUpper(string(string(property)[0])) + string(property)[1:] + "s"
	log.Debugf("FilterEntityType: %s, CloudProperty: %s, FilterConfigProperty: %s", typeProperty, cloudProperty, propertyProperty)

	if typeField := reflect.ValueOf(filterConfig).FieldByName(typeProperty); typeField.IsValid() && !typeField.IsNil() {
		if cloudField := reflect.Indirect(typeField).FieldByName(cloudProperty); cloudField.IsValid() {
			if propertyField := reflect.Indirect(cloudField).FieldByName(propertyProperty); propertyField.IsValid() {
				return propertyField.Interface().([]string)
			}
		}
	}
	return nil
}
