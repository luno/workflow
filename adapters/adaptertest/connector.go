package adaptertest

import (
	"context"
	"testing"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memstreamer"
)

type User struct {
	UID         string
	Email       string
	CountryCode string
}

func RunConnectorTest(t *testing.T, maker func(seedEvents []workflow.ConnectorEvent) workflow.ConnectorConstructor) {
	events := []workflow.ConnectorEvent{
		{
			ID:        "1",
			ForeignID: "SKDJF-2304893-SDFKJBSD",
		},
		{
			ID:        "2",
			ForeignID: "LSKDF-23OU5QL-23K45B23",
		},
	}
	constructor := maker(events)

	workflowName := "connector-test"
	builder := workflow.NewBuilder[User, SyncStatus](workflowName)
	builder.AddConnector(
		"tester",
		constructor,
		func(ctx context.Context, w *workflow.Workflow[User, SyncStatus], e *workflow.ConnectorEvent) error {
			_, err := w.Trigger(ctx, e.ForeignID, SyncStatusStarted, workflow.WithInitialValue[User, SyncStatus](&User{
				UID: e.ForeignID,
			}))
			if err != nil {
				return err
			}

			return nil
		},
	)

	builder.AddStep(SyncStatusStarted,
		func(ctx context.Context, r *workflow.Record[User, SyncStatus]) (SyncStatus, error) {
			r.Object.Email = "connector@workflow.com"
			return SyncStatusEmailSet, nil
		},
		SyncStatusEmailSet,
	)

	rs := memrecordstore.New()
	w := builder.Build(
		memstreamer.New(),
		rs,
		memrolescheduler.New(),
	)

	ctx := context.Background()
	w.Run(ctx)
	t.Cleanup(w.Stop)

	// Ensure a workflow was triggered for each of the events
	for _, e := range events {
		workflow.Require(t, w, e.ForeignID, SyncStatusEmailSet, User{
			UID:   e.ForeignID,
			Email: "connector@workflow.com",
		})
	}
}
