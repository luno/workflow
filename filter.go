package workflow

import (
	"slices"
	"strconv"
	"strings"
	"time"
)

const multiValueDelimiter = ","

func MakeFilter(filters ...RecordFilter) *recordFilters {
	var rf recordFilters
	for _, f := range filters {
		f(&rf)
	}

	return &rf
}

type recordFilters struct {
	byForeignID       Filter
	byStatus          Filter
	byRunState        Filter
	byCreatedAtAfter  FilterTime
	byCreatedAtBefore FilterTime
}

func (r recordFilters) ByForeignID() Filter {
	return r.byForeignID
}

func (r recordFilters) ByStatus() Filter {
	return r.byStatus
}

func (r recordFilters) ByRunState() Filter {
	return r.byRunState
}

func (r recordFilters) ByCreatedAtAfter() FilterTime {
	return r.byCreatedAtAfter
}
func (r recordFilters) ByCreatedAtBefore() FilterTime {
	return r.byCreatedAtBefore
}

func makeFilterValue(value string, isMultiMatch bool) Filter {
	return Filter{
		Enabled:      true,
		IsMultiMatch: isMultiMatch,
		value:        value,
	}
}

type Filter struct {
	Enabled      bool
	IsMultiMatch bool
	value        string
}

func (f Filter) Matches(findValue string) bool {
	if !f.IsMultiMatch {
		return f.value == findValue
	}

	return slices.Contains(f.MultiValues(), findValue)
}

func (f Filter) MultiValues() []string {
	return strings.Split(f.value, multiValueDelimiter)
}

func (f Filter) Value() string {
	return f.value
}

func makeFilterTime(compare int, value time.Time) FilterTime {
	return FilterTime{
		Enabled: true,
		compare: compare,
		value:   value,
	}
}

type FilterTime struct {
	Enabled bool
	compare int
	value   time.Time
}

func (f FilterTime) Matches(findValue time.Time) bool {
	if f.value.Compare(findValue) == f.compare {
		return true
	}
	return false
}
func (f FilterTime) Value() time.Time {
	return f.value
}

type RecordFilter func(filters *recordFilters)

func FilterByForeignID(foreignIDs ...string) RecordFilter {
	return func(filters *recordFilters) {
		if len(foreignIDs) == 1 {
			filters.byForeignID = makeFilterValue(foreignIDs[0], false)
			return
		}

		var val strings.Builder
		for i, foreignID := range foreignIDs {
			if i != 0 {
				val.WriteString(multiValueDelimiter)
			}

			val.WriteString(foreignID)
		}
		filters.byForeignID = makeFilterValue(val.String(), true)
	}
}

func FilterByStatus[statusType ~int | ~int8 | ~int16 | ~int32 | ~int64](statuses ...statusType) RecordFilter {
	return func(filters *recordFilters) {
		if len(statuses) == 1 {
			i := strconv.FormatInt(int64(statuses[0]), 10)
			filters.byStatus = makeFilterValue(i, false)
			return
		}

		var val strings.Builder
		for i, status := range statuses {
			if i != 0 {
				val.WriteString(multiValueDelimiter)
			}

			val.WriteString(strconv.FormatInt(int64(status), 10))
		}
		filters.byStatus = makeFilterValue(val.String(), true)
	}
}

func FilterByRunState(runStates ...RunState) RecordFilter {
	return func(filters *recordFilters) {
		if len(runStates) == 1 {
			i := strconv.FormatInt(int64(runStates[0]), 10)
			filters.byRunState = makeFilterValue(i, false)
			return
		}

		var val strings.Builder
		for i, rs := range runStates {
			if i != 0 {
				val.WriteString(multiValueDelimiter)
			}

			val.WriteString(strconv.FormatInt(int64(rs), 10))
		}
		filters.byRunState = makeFilterValue(val.String(), true)
	}
}

func FilterByCreatedAtAfter(after time.Time) RecordFilter {
	return func(filters *recordFilters) {
		filters.byCreatedAtAfter = makeFilterTime(-1, after)
	}
}

func FilterByCreatedAtBefore(before time.Time) RecordFilter {
	return func(filters *recordFilters) {
		filters.byCreatedAtBefore = makeFilterTime(1, before)
	}
}
