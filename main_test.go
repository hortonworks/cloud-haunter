package main

import (
	"testing"

	"github.com/hortonworks/cloud-haunter/types"
	"github.com/stretchr/testify/assert"
)

type stubFilter struct{}

func (stubFilter) Execute(items []types.CloudItem) ([]types.CloudItem, error) {
	return items, nil
}

func TestSelectFilters(t *testing.T) {
	filters := map[types.FilterType]types.Filter{
		types.LongRunningFilter: stubFilter{},
		types.IgnoredFilter:     stubFilter{},
	}
	cases := []struct {
		name          string
		list          string
		expectedNames []types.FilterType
		expectedErr   string
	}{
		{name: "no filter requested", list: ""},
		{name: "known filter", list: "longrunning", expectedNames: []types.FilterType{types.LongRunningFilter}},
		{name: "names are trimmed, case-insensitive and kept in order", list: " LongRunning ,IGNORED", expectedNames: []types.FilterType{types.LongRunningFilter, types.IgnoredFilter}},
		{name: "order follows the list", list: "ignored,longrunning", expectedNames: []types.FilterType{types.IgnoredFilter, types.LongRunningFilter}},
		{name: "unknown filter", list: "longruning", expectedErr: "filter is not found: longruning"},
		{name: "duplicates are applied once", list: "longrunning,LONGRUNNING", expectedNames: []types.FilterType{types.LongRunningFilter}},
		{name: "every unknown filter is reported", list: "ignored,longruning,igonred", expectedErr: "filter is not found: longruning, igonred"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			matchedFilters, filterNames, err := selectFilters(filters, tc.list)

			if tc.expectedErr != "" {
				assert.EqualError(t, err, tc.expectedErr)
				assert.Empty(t, matchedFilters)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedNames, filterNames)
			assert.Len(t, matchedFilters, len(tc.expectedNames))
		})
	}
}
