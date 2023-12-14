package reflexstreamer

type EventType int

func (ev EventType) ReflexType() int {
	return int(ev)
}
