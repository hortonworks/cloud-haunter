package utils

import (
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"

	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	"gopkg.in/yaml.v2"
)

// IsAnyMatch looks for any of the given tag in types.Tag
func IsAnyMatch(haystack map[string]string, needles ...string) bool {
	for _, n := range needles {
		if _, ok := haystack[n]; ok {
			return true
		}
	}
	return false
}

// IsAnyStartsWith looks any tag start with given needles
func IsAnyStartsWith(haystack map[string]string, needles ...string) bool {
	for k := range haystack {
		if IsStartsWith(k, needles...) {
			return true
		}
	}
	return false
}

// IsStartsWith looks input start with given needles
func IsStartsWith(hay string, needles ...string) bool {
	for _, n := range needles {
		if strings.Index(hay, n) == 0 {
			return true
		}
	}
	return false
}

// ConvertTimeRFC3339 converts RFC3339 format string to time.Time
func ConvertTimeRFC3339(stringTime string) (time.Time, error) {
	return time.Parse(time.RFC3339, stringTime)
}

// ConvertTimeUnix parses a unix timestamp (seconds since epoch start) from string to time.Time
func ConvertTimeUnix(unixTimestamp string) time.Time {
	timestamp, err := strconv.ParseInt(unixTimestamp, 10, 64)
	if err != nil {
		log.Warnf("[util.ConvertTimeUnix] cannot convert time: %s, err: %s", unixTimestamp, err)
		timestamp = 0
	}
	return time.Unix(timestamp, 0)
}

// ConvertTags converts a map of tags to types.Tag
func ConvertTags(tagMap map[string]*string) types.Tags {
	tags := make(types.Tags, 0)
	for k, v := range tagMap {
		tags[k] = *v
	}
	return tags
}

// LoadIgnores loads and unmarshalls ignore config YAML
func LoadIgnores(location string) (*types.Ignores, error) {
	raw, err := ioutil.ReadFile(location)
	if err != nil {
		return nil, err
	}
	ignores := &types.Ignores{}
	err = yaml.Unmarshal(raw, ignores)
	if err != nil {
		return nil, err
	}
	return ignores, nil
}

func GetCloudAccountNames() map[types.CloudType]string {
	var accounts = make(map[types.CloudType]string)
	for cType, initFunc := range ctx.CloudProviders {
		accounts[cType] = initFunc().GetAccountName()
	}
	return accounts
}
