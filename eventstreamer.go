package workflow

import (
	"context"
	"time"
)

// EventStreamer defines the event streaming adapter interface / api and all implementations should all be
// tested with adaptertest.TestEventStreamer to ensure the behaviour is compatible with workflow.
type EventStreamer interface {
	NewSender(ctx context.Context, topic string) (EventSender, error)
	NewReceiver(ctx context.Context, topic string, name string, opts ...ReceiverOption) (EventReceiver, error)
}

// EventSender defines the common interface that the EventStreamer adapter must implement for allowing the workflow
// to send events to the event streamer.
type EventSender interface {
	Send(ctx context.Context, foreignID string, statusType int, headers map[Header]string) error
	Close() error
}

// EventReceiver defines the common interface that the EventStreamer adapter must implement for allowing the workflow
// to receive events.
type EventReceiver interface {
	Recv(ctx context.Context) (*Event, Ack, error)
	Close() error
}

// Ack is used for the event streamer to safeUpdate its cursor of what messages have
// been consumed. If Ack is not called then the event streamer, depending on implementation,
// will likely not keep track of which records / events have been consumed.
type Ack func() error

type Header string

const (
	HeaderWorkflowName  Header = "workflow_name"
	HeaderForeignID     Header = "foreign_id"
	HeaderTopic         Header = "topic"
	HeaderRunID         Header = "run_id"
	HeaderRunState      Header = "run_state"
	HeaderRecordVersion Header = "record_version"
	HeaderConnectorData Header = "connector_data"
)

type ReceiverOptions struct {
	PollFrequency    time.Duration
	StreamFromLatest bool
}

type ReceiverOption func(*ReceiverOptions)

func WithReceiverPollFrequency(d time.Duration) ReceiverOption {
	return func(opt *ReceiverOptions) {
		opt.PollFrequency = d
	}
}

// StreamFromLatest tells the event streamer to start streaming events from the most recent event if there is no
// commited/stored offset (cursor for some event streaming platforms). If a consumer has received events before then
// this should have no affect and consumption should resume from where it left off previously.
func StreamFromLatest() ReceiverOption {
	return func(opt *ReceiverOptions) {
		opt.StreamFromLatest = true
	}
}
