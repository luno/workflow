package memrecordstore_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/adaptertest"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memstreamer"
	"github.com/luno/workflow/internal/logger"
)

func TestStore(t *testing.T) {
	adaptertest.RunRecordStoreTest(t, func() workflow.RecordStore {
		return memrecordstore.New()
	})
}

type status int

const (
	StatusUnknown status = 0
	StatusStart   status = 1
	StatusMiddle  status = 2
	StatusEnd     status = 3
)

func (s status) String() string {
	switch s {
	case StatusStart:
		return "Start"
	case StatusMiddle:
		return "Middle"
	case StatusEnd:
		return "End"
	default:
		return "Unknown"
	}
}

func TestOutboxDisabled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
	})

	eventStreamer := memstreamer.New()
	recordStore := memrecordstore.New(memrecordstore.WithOutbox(ctx, eventStreamer, logger.New(os.Stdout)))

	b := workflow.NewBuilder[string, status]("super fast workflow")
	b.AddStep(StatusStart, func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
		return StatusMiddle, nil
	}, StatusMiddle)
	b.AddStep(StatusMiddle, func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
		return StatusEnd, nil
	}, StatusEnd)
	wf := b.Build(
		eventStreamer,
		recordStore,
		memrolescheduler.New(),
		workflow.WithoutOutbox(),
	)

	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	runID, err := wf.Trigger(ctx, "some-related-id")
	require.Nil(t, err)

	_, err = wf.Await(ctx, "some-related-id", runID, StatusEnd)
	require.Nil(t, err)
}
