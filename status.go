package workflow

type StatusType interface {
	~int | ~int32 | ~int64

	String() string
}

func skipUpdate[Status StatusType](status Status) bool {
	return skipType(status) == skipTypeDefault || skipType(status) == skipTypeRunStateUpdate
}

func isSaveAndRepeat[Status StatusType](status Status) bool {
	return skipType(status) == skipTypeSaveAndRepeat
}

func skipUpdateDescription[Status StatusType](status Status) string {
	description, ok := skipConfig[skipType(status)]
	if !ok {
		return "Unknown skip reason '" + status.String() + "'"
	}

	return description
}

type skipType int

var (
	skipTypeDefault        skipType = 0
	skipTypeRunStateUpdate skipType = -1
	skipTypeSaveAndRepeat  skipType = -2
	skipTypeSentinel       skipType = -3
)

func allSkipTypes() []int {
	var all []int
	for i := skipTypeDefault; i > skipTypeSentinel; i-- {
		all = append(all, int(i))
	}

	return all
}

// skipConfig holds the skip values and descriptions as documentation as to what they mean.
var skipConfig = map[skipType]string{
	skipTypeDefault:        "Zero status with nil error value should result in a skip",
	skipTypeRunStateUpdate: "Internal run state update taken place. Skip normal newUpdater",
	skipTypeSaveAndRepeat:  "Save and repeat operation: record saved and re-queued for processing",
}
