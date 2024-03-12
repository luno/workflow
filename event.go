package workflow

import (
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/luno/workflow/workflowpb"
)

type Event struct {
	// ID is a unique ID for the event generated by the event streamer.
	ID int64

	// ForeignID refers to the ID of a record in the record store.
	ForeignID int64

	// Type relates to the StatusType that the associated record changed to.
	Type int

	// Headers stores meta-data in a simple and easily queryable way.
	Headers map[Header]string

	// CreatedAt is the time that the event was produced and is generated by the event streamer.
	CreatedAt time.Time
}

type OutboxEvent struct {
	// ID is a unique ID for this specific OutboxEvent.
	ID int64

	// WorkflowName refers to the name of the workflow that the OutboxEventData belongs to.
	WorkflowName string

	// Data represents a slice of bytes the OutboxEventDataMaker constructs via serialising event data
	// in an expected way for it to also be deserialized by the outbox consumer.
	Data []byte

	// CreatedAt is the time that this specific OutboxEvent was produced.
	CreatedAt time.Time
}

type OutboxEventData struct {
	// WorkflowName refers to the name of the workflow that the OutboxEventData belongs to.
	WorkflowName string

	// Data represents a slice of bytes the OutboxEventDataMaker constructs via serialising event data
	// in an expected way for it to also be deserialized by the outbox consumer.
	Data []byte
}

func WireRecordToOutboxEventData(record WireRecord) (OutboxEventData, error) {
	topic := Topic(record.WorkflowName, record.Status)

	headers := make(map[string]string)
	headers[string(HeaderWorkflowForeignID)] = record.ForeignID
	headers[string(HeaderWorkflowName)] = record.WorkflowName
	headers[string(HeaderTopic)] = topic
	headers[string(HeaderRunID)] = record.RunID

	r := workflowpb.OutboxRecord{
		ForeignId: record.ID,
		Type:      int32(record.Status),
		Headers:   headers,
	}

	data, err := proto.Marshal(&r)
	if err != nil {
		return OutboxEventData{}, err
	}

	return OutboxEventData{
		WorkflowName: record.WorkflowName,
		Data:         data,
	}, nil
}

type Header string

const (
	HeaderWorkflowName      Header = "workflow_name"
	HeaderWorkflowForeignID Header = "workflow_foreign_id"
	HeaderTopic             Header = "topic"
	HeaderRunID             Header = "run_id"
)
