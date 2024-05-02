package workflow

import "strconv"

func MakeFilter(filters ...RecordFilter) *recordFilters {
	var rf recordFilters
	for _, f := range filters {
		f(&rf)
	}

	return &rf
}

type recordFilters struct {
	byForeignID FilterValue
	byStatus    FilterValue
	byRunState  FilterValue
}

func (r recordFilters) ByForeignID() FilterValue {
	return r.byForeignID
}

func (r recordFilters) ByStatus() FilterValue {
	return r.byStatus
}

func (r recordFilters) ByRunState() FilterValue {
	return r.byRunState
}

func makeFilterValue(value string) FilterValue {
	return FilterValue{
		Enabled: true,
		Value:   value,
	}
}

type FilterValue struct {
	Enabled bool
	Value   string
}

type RecordFilter func(filters *recordFilters)

func FilterByForeignID(val string) RecordFilter {
	return func(filters *recordFilters) {
		filters.byForeignID = makeFilterValue(val)
	}
}

func FilterByStatus[status StatusType](s status) RecordFilter {
	return func(filters *recordFilters) {
		i := strconv.FormatInt(int64(s), 10)
		filters.byStatus = makeFilterValue(i)
	}
}

func FilterByRunState(rs RunState) RecordFilter {
	return func(filters *recordFilters) {
		i := strconv.FormatInt(int64(rs), 10)
		filters.byRunState = makeFilterValue(i)
	}
}
