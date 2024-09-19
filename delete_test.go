package workflow_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/clock"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memstreamer"
)

func TestDeleteForever(t *testing.T) {
	type object struct {
		pii    string
		notPII string
	}

	testErr := errors.New("test error")

	testCases := []struct {
		Name        string
		storeFn     func(ctx context.Context, record *workflow.Record, maker workflow.OutboxEventDataMaker) error
		lookupFn    func(ctx context.Context, id int64) (*workflow.Record, error)
		deleteFn    func(wr *workflow.Record) ([]byte, error)
		expectedErr error
	}{
		{
			Name: "Golden path - custom delete",
			storeFn: func(ctx context.Context, record *workflow.Record, maker workflow.OutboxEventDataMaker) error {
				require.Equal(t, workflow.RunStateDataDeleted, record.RunState)
				return nil
			},
			lookupFn: func(ctx context.Context, id int64) (*workflow.Record, error) {
				require.Equal(t, int64(1), id)

				o := object{
					pii:    "my name",
					notPII: "name of the month",
				}

				b, err := workflow.Marshal(&o)
				require.Nil(t, err)

				return &workflow.Record{
					Object:   b,
					RunState: workflow.RunStateRequestedDataDeleted,
				}, nil
			},
			deleteFn: func(wr *workflow.Record) ([]byte, error) {
				var o object

				err := workflow.Unmarshal(wr.Object, &o)
				require.Nil(t, err)

				o.pii = ""

				return workflow.Marshal(&o)
			},
			expectedErr: nil,
		},
		{
			Name: "Golden path - default delete",
			storeFn: func(ctx context.Context, record *workflow.Record, maker workflow.OutboxEventDataMaker) error {
				require.Equal(t, workflow.RunStateDataDeleted, record.RunState)
				return nil
			},
			lookupFn: func(ctx context.Context, id int64) (*workflow.Record, error) {
				require.Equal(t, int64(1), id)

				o := object{
					pii:    "my name",
					notPII: "name of the month",
				}

				b, err := workflow.Marshal(&o)
				require.Nil(t, err)

				return &workflow.Record{
					Object:   b,
					RunState: workflow.RunStateRequestedDataDeleted,
				}, nil
			},
			expectedErr: nil,
		},
		{
			Name: "Return err on lookup error",
			lookupFn: func(ctx context.Context, id int64) (*workflow.Record, error) {
				return nil, testErr
			},
			expectedErr: testErr,
		},
		{
			Name: "Return err on store error",
			lookupFn: func(ctx context.Context, id int64) (*workflow.Record, error) {
				return &workflow.Record{
					RunState: workflow.RunStateRequestedDataDeleted,
				}, nil
			},
			storeFn: func(ctx context.Context, record *workflow.Record, maker workflow.OutboxEventDataMaker) error {
				return testErr
			},
			expectedErr: testErr,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()
			streamer := memstreamer.New()
			workflowName := "example"

			producer, err := streamer.NewProducer(ctx, workflow.DeleteTopic(workflowName))
			require.Nil(t, err)
			t.Cleanup(func() {
				producer.Close()
			})

			recordID := int64(1)
			err = producer.Send(ctx, recordID, 1, map[workflow.Header]string{
				workflow.HeaderWorkflowName:     workflowName,
				workflow.HeaderForeignID:        "1",
				workflow.HeaderTopic:            workflow.DeleteTopic(workflowName),
				workflow.HeaderRunState:         workflow.RunStateRequestedDataDeleted.String(),
				workflow.HeaderPreviousRunState: workflow.RunStateCompleted.String(),
			})
			require.Nil(t, err)

			consumer, err := streamer.NewConsumer(ctx, workflow.DeleteTopic(workflowName), "consumer-1")
			require.Nil(t, err)
			t.Cleanup(func() {
				consumer.Close()
			})

			err = workflow.DeleteForever(ctx, workflowName, "process-name", consumer, tc.storeFn, tc.lookupFn, tc.deleteFn, time.Hour, clock.RealClock{})
			require.True(t, errors.Is(err, tc.expectedErr))
		})
	}
}
