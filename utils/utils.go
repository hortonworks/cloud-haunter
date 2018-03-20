package utils

import (
	"time"

	"github.com/hortonworks/cloud-cost-reducer/types"
)

func IsAnyMatch(haystack map[string]string, needle ...string) bool {
	for _, k := range needle {
		if _, ok := haystack[k]; ok {
			return true
		}
	}
	return false
}

func ConvertTimeRFC3339(stringTime string) (time.Time, error) {
	return time.Parse(time.RFC3339, stringTime)
}

func ConvertTags(tagMap map[string]*string) types.Tags {
	tags := make(types.Tags, 0)
	for k, v := range tagMap {
		tags[k] = *v
	}
	return tags
}
