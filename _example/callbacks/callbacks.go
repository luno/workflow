package callbacks

import (
	"context"
	"encoding/json"
	"io"

	"github.com/luno/workflow"
	"github.com/luno/workflow/example"
)

type Example struct {
	EmailConfirmed bool
}

type EmailConfirmationResponse struct {
	Confirmed bool
}

type Deps struct {
	EventStreamer workflow.EventStreamer
	RecordStore   workflow.RecordStore
	TimeoutStore  workflow.TimeoutStore
	RoleScheduler workflow.RoleScheduler
}

func ExampleWorkflow(d Deps) *workflow.Workflow[Example, example.Status] {
	b := workflow.NewBuilder[Example, example.Status]("callback example")

	b.AddCallback(example.StatusStarted, func(ctx context.Context, r *workflow.Record[Example, example.Status], reader io.Reader) (example.Status, error) {
		b, err := io.ReadAll(reader)
		if err != nil {
			return 0, err
		}

		var e EmailConfirmationResponse
		err = json.Unmarshal(b, &e)
		if err != nil {
			return 0, err
		}

		r.Object.EmailConfirmed = e.Confirmed

		return example.StatusFollowedTheExample, nil
	}, example.StatusFollowedTheExample)

	return b.Build(
		d.EventStreamer,
		d.RecordStore,
		d.RoleScheduler,
	)
}
