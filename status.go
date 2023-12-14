package workflow

type StatusType interface {
	~int | ~int32 | ~int64

	String() string
}
