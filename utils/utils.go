package utils

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"encoding/json"

	ctx "github.com/hortonworks/cloud-haunter/context"
	"github.com/hortonworks/cloud-haunter/types"
	"gopkg.in/yaml.v2"
)

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

// IsAnyEquals returns if the input string equals to any of the array elements
func IsAnyEquals(input string, array ...string) bool {
	for _, s := range array {
		if input == s {
			return true
		}
	}
	return false
}

// ConvertTimeRFC3339 converts RFC3339 format string to time.Time
func ConvertTimeRFC3339(stringTime string) (time.Time, error) {
	return time.Parse(time.RFC3339, stringTime)
}

// ConvertTimeLayout converts a string in the format of the layout to time.Time
func ConvertTimeLayout(layout, timeString string) (time.Time, error) {
	return time.Parse(layout, timeString)
}

// ConvertTimeUnix parses a unix timestamp (seconds since epoch start) from string to time.Time
func ConvertTimeUnix(unixTimestamp string) time.Time {
	timestamp, err := strconv.ParseInt(unixTimestamp, 10, 64)
	if err != nil {
		log.Warnf("[util.ConvertTimeUnix] cannot convert time: %s, err: %s", unixTimestamp, err)
		timestamp = 0
	}
	t := time.Unix(timestamp, 0)
	if t.Year() >= 3000 || t.Year() <= 1970 {
		t = time.Unix(0, timestamp*int64(1000000))
	}
	return t
}

// ConvertTags converts a map of tags to types.Tag
func ConvertTags(tagMap map[string]*string) types.Tags {
	tags := make(types.Tags, 0)
	for k, v := range tagMap {
		tags[k] = *v
	}
	return tags
}

// LoadFilterConfig loads and unmarshalls filter config YAML
func LoadFilterConfig(location string) (*types.FilterConfig, error) {
	raw, err := ioutil.ReadFile(location)
	if err != nil {
		return nil, err
	}
	config := &types.FilterConfig{}
	err = yaml.UnmarshalStrict(raw, config)
	if err != nil {
		return nil, err
	}
	log.Debugf("[UTIL] Filter config loaded:\n%s", raw)
	return config, nil
}

func LoadFilterConfigV2(location string) (*types.FilterConfigV2, error) {
	raw, err := ioutil.ReadFile(location)
	if err != nil {
		return nil, err
	}
	configV2 := &types.FilterConfigV2{}
	err = yaml.UnmarshalStrict(raw, configV2)
	if err != nil {
		return nil, err
	}
	log.Debugf("[UTIL] Filter config V2 loaded:\n%s", raw)
	return configV2, nil
}

// GetCloudAccountNames returns the name of the configured cloud accounts
func GetCloudAccountNames() map[types.CloudType]string {
	var accounts = make(map[types.CloudType]string)
	for cType, initFunc := range ctx.CloudProviders {
		accounts[cType] = initFunc().GetAccountName()
	}
	return accounts
}

// SplitListToMap splits comma separated list to key:true map
func SplitListToMap(list string) (resp map[string]bool) {
	resp = map[string]bool{}
	for _, i := range strings.Split(list, ",") {
		if len(i) != 0 {
			trimmed := strings.Trim(i, " ")
			resp[strings.ToLower(trimmed)] = true
			resp[strings.ToUpper(trimmed)] = true
		}
	}
	return
}

// CovertJsonToString converts a struct to json string
func CovertJsonToString(source interface{}) (*string, error) {
	j, err := json.Marshal(source)
	if err != nil {
		return nil, err
	}
	return &(&types.S{S: string(j)}).S, nil
}

// GetFilterNames returns the name of the applied filters separated by colon
func GetFilterNames(filters []types.FilterType) string {
	if len(filters) == 0 {
		return "noFilter"
	}
	fNames := make([]string, 0)
	for _, f := range filters {
		fNames = append(fNames, f.String())
	}
	return strings.Join(fNames, ",")
}

func GetHumanReadableFileSize(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(b)/float64(div), "kMGTPE"[exp])
}
