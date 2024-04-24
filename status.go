package workflow

type StatusType interface {
	~int | ~int32 | ~int64

	String() string
}

func skipUpdate[Status StatusType](status Status) bool {
	_, ok := skipConfig[SkipType(status)]
	return ok
}

func skipUpdateDescription[Status StatusType](status Status) string {
	description, ok := skipConfig[SkipType(status)]
	if !ok {
		return "Unknown skip reason '" + status.String() + "'"
	}

	return description
}

type SkipType int

var (
	SkipTypeDefault        SkipType = 0
	SkipTypeRunStateUpdate SkipType = -1
)

// skipConfig holds the skip values and descriptions as documentation as to what they mean.
var skipConfig = map[SkipType]string{
	SkipTypeDefault:        "Zero status with nil error value should result in a skip",
	SkipTypeRunStateUpdate: "Internal run state update taken place. Skip normal newUpdater",
}
