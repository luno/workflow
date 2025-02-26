package workflow

import (
	"strconv"
	"strings"
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
	byForeignID Filter
	byStatus    Filter
	byRunState  Filter
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

	for _, filterValue := range f.MultiValues() {
		if filterValue == findValue {
			return true
		}
	}

	return false
}

func (f Filter) MultiValues() []string {
	return strings.Split(f.value, multiValueDelimiter)
}

func (f Filter) Value() string {
	return f.value
}

type RecordFilter func(filters *recordFilters)

func FilterByForeignID(foreignIDs ...string) RecordFilter {
	return func(filters *recordFilters) {
		if len(foreignIDs) == 1 {
			filters.byForeignID = makeFilterValue(foreignIDs[0], false)
			return
		}

		var val string
		for i, foreignID := range foreignIDs {
			if i != 0 {
				val += multiValueDelimiter
			}

			val += foreignID
		}
		filters.byForeignID = makeFilterValue(val, true)
	}
}

func FilterByStatus[statusType ~int | ~int8 | ~int16 | ~int32 | ~int64](statuses ...statusType) RecordFilter {
	return func(filters *recordFilters) {
		if len(statuses) == 1 {
			i := strconv.FormatInt(int64(statuses[0]), 10)
			filters.byStatus = makeFilterValue(i, false)
			return
		}

		var val string
		for i, status := range statuses {
			if i != 0 {
				val += multiValueDelimiter
			}

			val += strconv.FormatInt(int64(status), 10)
		}
		filters.byStatus = makeFilterValue(val, true)
	}
}

func FilterByRunState(runStates ...RunState) RecordFilter {
	return func(filters *recordFilters) {
		if len(runStates) == 1 {
			i := strconv.FormatInt(int64(runStates[0]), 10)
			filters.byRunState = makeFilterValue(i, false)
			return
		}

		var val string
		for i, rs := range runStates {
			if i != 0 {
				val += multiValueDelimiter
			}

			val += strconv.FormatInt(int64(rs), 10)
		}
		filters.byRunState = makeFilterValue(val, true)
	}
}
