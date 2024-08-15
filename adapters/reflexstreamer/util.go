package reflexstreamer

import (
	"strconv"

	"github.com/luno/reflex"

	"github.com/luno/workflow"
)

type EventType int

func (ev EventType) ReflexType() int {
	return int(ev)
}

var HeaderMeta = "meta"

func DefaultReflexTranslator(e *reflex.Event) (*workflow.ConnectorEvent, error) {
	return &workflow.ConnectorEvent{
		ID:        e.ID,
		ForeignID: e.ForeignID,
		Type:      strconv.FormatInt(int64(e.Type.ReflexType()), 10),
		Headers: map[string]string{
			HeaderMeta: string(e.MetaData),
		},
		CreatedAt: e.Timestamp,
	}, nil
}

func DefaultConnectorTranslator(e *workflow.ConnectorEvent) (*reflex.Event, error) {
	var meta []byte
	if data, ok := e.Headers[HeaderMeta]; ok {
		meta = []byte(data)
	}

	return &reflex.Event{
		ID:        e.ID,
		Type:      reflexType(e.Type),
		ForeignID: e.ForeignID,
		Timestamp: e.CreatedAt,
		MetaData:  meta,
	}, nil
}

type reflexType string

func (rt reflexType) ReflexType() int {
	i, _ := strconv.ParseInt(string(rt), 10, 64)
	return int(i)
}
