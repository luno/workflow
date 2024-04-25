package workflow

type OrderType int

const (
	OrderTypeUnknown    OrderType = 0
	OrderTypeAscending  OrderType = 1
	OrderTypeDescending OrderType = 2
	sentinelOrderType   OrderType = 3
)

func (ot OrderType) String() string {
	switch ot {
	case OrderTypeAscending:
		return "asc"

	case OrderTypeDescending:
		return "desc"
	default:
		return ""
	}
}
