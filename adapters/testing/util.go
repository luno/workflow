package testing

type status int

const (
	statusUnknown status = 0
	statusStarted status = 1
	statusMiddle  status = 2
	statusEnd     status = 3
)

func (s status) String() string {
	switch s {
	case statusStarted:
		return "Started"
	case statusMiddle:
		return "Middle"
	case statusEnd:
		return "End"
	default:
		return "Unknown"
	}
}
