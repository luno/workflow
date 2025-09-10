package adaptertest

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	clock_testing "k8s.io/utils/clock/testing"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memtimeoutstore"
)

type SyncStatus int

const (
	SyncStatusUnknown           SyncStatus = 0
	SyncStatusStarted           SyncStatus = 1
	SyncStatusEmailSet          SyncStatus = 2
	SyncStatusRegulationTimeout SyncStatus = 3
	SyncStatusCompleted         SyncStatus = 4
)

func (s SyncStatus) String() string {
	switch s {
	case SyncStatusStarted:
		return "Started"
	case SyncStatusEmailSet:
		return "Email set"
	case SyncStatusRegulationTimeout:
		return "Regulatory cool down period"
	case SyncStatusCompleted:
		return "Completed"
	default:
		return "Unknown"
	}
}

func RunEventStreamerTest(t *testing.T, factory func() workflow.EventStreamer) {
	t.Run("ReceiverOption - Ensure StreamFromLatest is implemented", func(t *testing.T) {
		streamer := factory()
		ctx := context.Background()
		topic := "test-1"
		sender, err := streamer.NewSender(ctx, topic)
		require.NoError(t, err)

		err = sender.Send(ctx, "123", 4, map[workflow.Header]string{
			workflow.HeaderTopic: topic,
		})
		require.NoError(t, err)

		err = sender.Send(ctx, "456", 5, map[workflow.Header]string{
			workflow.HeaderTopic: topic,
		})
		require.NoError(t, err)

		var wg sync.WaitGroup

		receiver, err := streamer.NewReceiver(ctx, topic, "my-receiver", workflow.StreamFromLatest())
		require.NoError(t, err)

		t.Run("Should only receive events that come in after connecting", func(t *testing.T) {
			wg.Add(1)
			go func() {
				go func() {
					err = sender.Send(ctx, "789", 5, map[workflow.Header]string{
						workflow.HeaderTopic: topic,
					})
					require.NoError(t, err)
				}()

				e, ack, err := receiver.Recv(ctx)
				require.NoError(t, err)
				require.Equal(t, "789", e.ForeignID)

				err = ack()
				require.NoError(t, err)

				wg.Done()
			}()

			wg.Wait()
		})

		err = receiver.Close()
		require.NoError(t, err)

		t.Run("StreamFromLatest should have no affect when offset is committed", func(t *testing.T) {
			err = sender.Send(ctx, "101", 5, map[workflow.Header]string{
				workflow.HeaderTopic: topic,
			})
			require.NoError(t, err)

			secondReceiver, err := streamer.NewReceiver(ctx, topic, "my-receiver", workflow.StreamFromLatest())
			require.NoError(t, err)

			// Should receive event send when receiver wasn't receiving events based on the offset being set.
			e, ack, err := secondReceiver.Recv(ctx)
			require.NoError(t, err)
			require.Equal(t, "101", e.ForeignID)

			err = ack()
			require.NoError(t, err)
		})
	})

	t.Run("Acceptance test - full workflow run through", func(t *testing.T) {
		b := workflow.NewBuilder[User, SyncStatus]("sync user 2")
		b.AddStep(
			SyncStatusStarted,
			setEmail(),
			SyncStatusEmailSet,
		).WithOptions(
			workflow.PollingFrequency(time.Millisecond*200),
			workflow.ParallelCount(5),
		)
		b.AddTimeout(
			SyncStatusEmailSet,
			coolDownTimerFunc(),
			coolDownTimeout(),
			SyncStatusRegulationTimeout,
		).WithOptions(
			workflow.PollingFrequency(time.Millisecond * 200),
		)
		b.AddStep(
			SyncStatusRegulationTimeout,
			generateUserID(),
			SyncStatusCompleted,
		).WithOptions(
			workflow.PollingFrequency(time.Millisecond*200),
			workflow.ParallelCount(5),
		)

		now := time.Date(2023, time.April, 9, 8, 30, 0, 0, time.UTC)
		clock := clock_testing.NewFakeClock(now)

		wf := b.Build(
			factory(),
			memrecordstore.New(),
			memrolescheduler.New(),
			workflow.WithClock(clock),
			workflow.WithTimeoutStore(memtimeoutstore.New()),
		)

		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(func() {
			cancel()
		})
		wf.Run(ctx)
		t.Cleanup(wf.Stop)

		foreignID := "1"
		u := User{
			CountryCode: "GB",
		}
		runId, err := wf.Trigger(ctx, foreignID, workflow.WithInitialValue[User, SyncStatus](&u))
		require.NoError(t, err)

		workflow.AwaitTimeoutInsert(t, wf, foreignID, runId, SyncStatusEmailSet)

		clock.Step(time.Hour)

		record, err := wf.Await(ctx, foreignID, runId, SyncStatusCompleted)
		require.NoError(t, err)

		require.Equal(t, "andrew@workflow.com", record.Object.Email)
		require.Equal(t, SyncStatusCompleted.String(), record.Status.String())
		require.NotEmpty(t, record.Object.UID)
	})
}

func setEmail() func(ctx context.Context, t *workflow.Run[User, SyncStatus]) (SyncStatus, error) {
	return func(ctx context.Context, t *workflow.Run[User, SyncStatus]) (SyncStatus, error) {
		t.Object.Email = "andrew@workflow.com"
		return SyncStatusEmailSet, nil
	}
}

func coolDownTimerFunc() func(ctx context.Context, r *workflow.Run[User, SyncStatus], now time.Time) (time.Time, error) {
	return func(ctx context.Context, r *workflow.Run[User, SyncStatus], now time.Time) (time.Time, error) {
		// Place a 1-hour cool down period for Great Britain users
		if r.Object.CountryCode == "GB" {
			return now.Add(time.Hour), nil
		}

		// Don't provide a timeout for users outside of GB
		return time.Time{}, nil
	}
}

func coolDownTimeout() func(ctx context.Context, r *workflow.Run[User, SyncStatus], now time.Time) (SyncStatus, error) {
	return func(ctx context.Context, r *workflow.Run[User, SyncStatus], now time.Time) (SyncStatus, error) {
		if r.Object.Email == "andrew@workflow.com" {
			return SyncStatusRegulationTimeout, nil
		}

		return 0, nil
	}
}

func generateUserID() func(ctx context.Context, t *workflow.Run[User, SyncStatus]) (SyncStatus, error) {
	return func(ctx context.Context, t *workflow.Run[User, SyncStatus]) (SyncStatus, error) {
		t.Object.UID = uuid.New().String()
		return SyncStatusCompleted, nil
	}
}
